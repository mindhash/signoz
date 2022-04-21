package alertManager

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net"
	"path"
	"strings"
	"sync/atomic"

	"net/http"
	"net/url"
	"sync"
	"time"

	old_ctx "golang.org/x/net/context"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/log/level"

	"github.com/prometheus/client_golang/prometheus"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"

	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/relabel"
	"golang.org/x/net/context/ctxhttp"
)

const (
	alertPushEndpoint = "/api/v1/alerts"
	contentTypeJSON   = "application/json"
)

// Alert is a generic representation of an alert in the Prometheus eco-system.
type Alert struct {
	// Label value pairs for purpose of aggregation, matching, and disposition
	// dispatching. This must minimally include an "alertname" label.
	Labels labels.Labels `json:"labels"`

	// Extra key/value information which does not define alert identity.
	Annotations labels.Labels `json:"annotations"`

	// The known time range for this alert. Both ends are optional.
	StartsAt     time.Time `json:"startsAt,omitempty"`
	EndsAt       time.Time `json:"endsAt,omitempty"`
	GeneratorURL string    `json:"generatorURL,omitempty"`
}

// Name returns the name of the alert. It is equivalent to the "alertname" label.
func (a *Alert) Name() string {
	return a.Labels.Get(labels.AlertName)
}

// Hash returns a hash over the alert. It is equivalent to the alert labels hash.
func (a *Alert) Hash() uint64 {
	return a.Labels.Hash()
}

func (a *Alert) String() string {
	s := fmt.Sprintf("%s[%s]", a.Name(), fmt.Sprintf("%016x", a.Hash())[:7])
	if a.Resolved() {
		return s + "[resolved]"
	}
	return s + "[active]"
}

// Resolved returns true iff the activity interval ended in the past.
func (a *Alert) Resolved() bool {
	return a.ResolvedAt(time.Now())
}

// ResolvedAt returns true off the activity interval ended before
// the given timestamp.
func (a *Alert) ResolvedAt(ts time.Time) bool {
	if a.EndsAt.IsZero() {
		return false
	}
	return !a.EndsAt.After(ts)
}

// Notifier is responsible for dispatching alert notifications to an
// alert manager service.
type Notifier struct {
	queue []*Alert
	opts  *NotifierOptions

	more   chan struct{}
	mtx    sync.RWMutex
	ctx    context.Context
	cancel func()

	alertmanagers map[string]*alertmanagerSet
	logger        log.Logger
}

// NotifierOptions are the configurable parameters of a Handler.
type NotifierOptions struct {
	QueueCapacity  int
	ExternalLabels model.LabelSet
	RelabelConfigs []*config.RelabelConfig
	// Used for sending HTTP requests to the Alertmanager.
	Do func(ctx old_ctx.Context, client *http.Client, req *http.Request) (*http.Response, error)

	Registerer prometheus.Registerer
}

// todo(amol): add metrics

func NewNotifier(o *NotifierOptions, logger log.Logger) *Notifier {
	ctx, cancel := context.WithCancel(context.Background())
	if o.Do == nil {
		o.Do = ctxhttp.Do
	}
	if logger == nil {
		logger = log.NewNopLogger()
	}

	n := &Notifier{
		queue:  make([]*Alert, 0, o.QueueCapacity),
		ctx:    ctx,
		cancel: cancel,
		more:   make(chan struct{}, 1),
		opts:   o,
		logger: logger,
	}

	return n

}

// ApplyConfig updates the status state as the new config requires.
func (n *Notifier) ApplyConfig(conf *config.Config) error {
	n.mtx.Lock()
	defer n.mtx.Unlock()

	n.opts.ExternalLabels = conf.GlobalConfig.ExternalLabels
	n.opts.RelabelConfigs = conf.AlertingConfig.AlertRelabelConfigs

	amSets := make(map[string]*alertmanagerSet)

	for _, cfg := range conf.AlertingConfig.AlertmanagerConfigs {
		ams, err := newAlertmanagerSet(cfg, n.logger)
		if err != nil {
			return err
		}

		// ams.metrics = n.metrics

		// The config hash is used for the map lookup identifier.
		b, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		amSets[fmt.Sprintf("%x", md5.Sum(b))] = ams
	}

	n.alertmanagers = amSets

	return nil
}

const maxBatchSize = 64

func (n *Notifier) queueLen() int {
	n.mtx.RLock()
	defer n.mtx.RUnlock()

	return len(n.queue)
}

func (n *Notifier) nextBatch() []*Alert {
	n.mtx.Lock()
	defer n.mtx.Unlock()

	var alerts []*Alert

	if len(n.queue) > maxBatchSize {
		alerts = append(make([]*Alert, 0, maxBatchSize), n.queue[:maxBatchSize]...)
		n.queue = n.queue[maxBatchSize:]
	} else {
		alerts = append(make([]*Alert, 0, len(n.queue)), n.queue...)
		n.queue = n.queue[:0]
	}

	return alerts
}

// Run dispatches notifications continuously.
func (n *Notifier) Run(tsets <-chan map[string][]*targetgroup.Group) {

	for {
		select {
		case <-n.ctx.Done():
			return
		case ts := <-tsets:
			n.reload(ts)
		case <-n.more:
		}
		alerts := n.nextBatch()

		if !n.sendAll(alerts...) {
			// n.metrics.dropped.Add(float64(len(alerts)))
		}
		// If the queue still has items left, kick off the next iteration.
		if n.queueLen() > 0 {
			n.setMore()
		}
	}
}

func (n *Notifier) reload(tgs map[string][]*targetgroup.Group) {
	n.mtx.Lock()
	defer n.mtx.Unlock()

	for id, tgroup := range tgs {
		am, ok := n.alertmanagers[id]
		if !ok {
			level.Error(n.logger).Log("msg", "couldn't sync alert manager set", "err", fmt.Sprintf("invalid id:%v", id))
			continue
		}
		am.sync(tgroup)
	}
}

// Send queues the given notification requests for processing.
// Panics if called on a handler that is not running.
func (n *Notifier) Send(alerts ...*Alert) {
	n.mtx.Lock()
	defer n.mtx.Unlock()

	// Attach external labels before relabelling and sending.
	for _, a := range alerts {
		lb := labels.NewBuilder(a.Labels)

		for ln, lv := range n.opts.ExternalLabels {
			if a.Labels.Get(string(ln)) == "" {
				lb.Set(string(ln), string(lv))
			}
		}

		a.Labels = lb.Labels()
	}

	alerts = n.relabelAlerts(alerts)

	// Queue capacity should be significantly larger than a single alert
	// batch could be.
	if d := len(alerts) - n.opts.QueueCapacity; d > 0 {
		alerts = alerts[d:]

		level.Warn(n.logger).Log("msg", "Alert batch larger than queue capacity, dropping alerts", "num_dropped", d)
		//n.metrics.dropped.Add(float64(d))
	}

	// If the queue is full, remove the oldest alerts in favor
	// of newer ones.
	if d := (len(n.queue) + len(alerts)) - n.opts.QueueCapacity; d > 0 {
		n.queue = n.queue[d:]

		level.Warn(n.logger).Log("msg", "Alert notification queue full, dropping alerts", "num_dropped", d)
		//n.metrics.dropped.Add(float64(d))
	}
	n.queue = append(n.queue, alerts...)

	// Notify sending goroutine that there are alerts to be processed.
	n.setMore()
}

func (n *Notifier) relabelAlerts(alerts []*Alert) []*Alert {
	var relabeledAlerts []*Alert

	for _, alert := range alerts {
		lset := make(model.LabelSet, 0)
		for _, l := range alert.Labels {
			lname := model.LabelName(l.Name)
			lvalue := model.LabelValue(l.Value)
			lset[lname] = lvalue
		}
		relabledset := relabel.Process(lset, n.opts.RelabelConfigs...)
		if relabledset != nil {
			var newLabels labels.Labels
			for name, val := range relabledset {
				l := labels.Label{
					Name: string(name), Value: string(val),
				}
				newLabels = append(newLabels, l)
			}
			alert.Labels = newLabels
			relabeledAlerts = append(relabeledAlerts, alert)
		}
	}
	return relabeledAlerts
}

// setMore signals that the alert queue has items.
func (n *Notifier) setMore() {
	// If we cannot send on the channel, it means the signal already exists
	// and has not been consumed yet.
	select {
	case n.more <- struct{}{}:
	default:
	}
}

// Alertmanagers returns a slice of Alertmanager URLs.
func (n *Notifier) Alertmanagers() []*url.URL {
	n.mtx.RLock()
	amSets := n.alertmanagers
	n.mtx.RUnlock()

	var res []*url.URL

	for _, ams := range amSets {
		ams.mtx.RLock()
		for _, am := range ams.ams {
			res = append(res, am.url())
		}
		ams.mtx.RUnlock()
	}

	return res
}

// DroppedAlertmanagers returns a slice of Alertmanager URLs.
func (n *Notifier) DroppedAlertmanagers() []*url.URL {
	n.mtx.RLock()
	amSets := n.alertmanagers
	n.mtx.RUnlock()

	var res []*url.URL

	for _, ams := range amSets {
		ams.mtx.RLock()
		for _, dam := range ams.droppedAms {
			res = append(res, dam.url())
		}
		ams.mtx.RUnlock()
	}

	return res
}

// sendAll sends the alerts to all configured Alertmanagers concurrently.
// It returns true if the alerts could be sent successfully to at least one Alertmanager.
func (n *Notifier) sendAll(alerts ...*Alert) bool {

	b, err := json.Marshal(alerts)
	if err != nil {
		level.Error(n.logger).Log("msg", "Encoding alerts failed", "err", err)
		return false
	}

	n.mtx.RLock()
	amSets := n.alertmanagers
	n.mtx.RUnlock()

	var (
		wg         sync.WaitGroup
		numSuccess uint64
	)
	for _, ams := range amSets {
		ams.mtx.RLock()

		for _, am := range ams.ams {
			wg.Add(1)

			ctx, cancel := context.WithTimeout(n.ctx, time.Duration(ams.cfg.Timeout))
			defer cancel()

			go func(ams *alertmanagerSet, am alertmanager) {
				u := am.url().String()

				if err := n.sendOne(ctx, ams.client, u, b); err != nil {
					level.Error(n.logger).Log("alertmanager", u, "count", len(alerts), "msg", "Error sending alert", "err", err)
					//n.metrics.errors.WithLabelValues(u).Inc()
				} else {
					atomic.AddUint64(&numSuccess, 1)
				}
				// n.metrics.latency.WithLabelValues(u).Observe(time.Since(begin).Seconds())
				// n.metrics.sent.WithLabelValues(u).Add(float64(len(alerts)))

				wg.Done()
			}(ams, am)
		}
		ams.mtx.RUnlock()
	}
	wg.Wait()

	return numSuccess > 0
}

func (n *Notifier) sendOne(ctx context.Context, c *http.Client, url string, b []byte) error {
	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentTypeJSON)
	resp, err := n.opts.Do(ctx, c, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Any HTTP status 2xx is OK.
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("bad response status %v", resp.Status)
	}
	return err
}

// Stop shuts down the notification handler.
func (n *Notifier) Stop() {
	level.Info(n.logger).Log("msg", "Stopping notification manager...")
	n.cancel()
}

// alertmanager holds Alertmanager endpoint information.
type alertmanager interface {
	url() *url.URL
}

type alertmanagerLabels struct{ labels.Labels }

const pathLabel = "__alerts_path__"

func (a alertmanagerLabels) url() *url.URL {
	return &url.URL{
		Scheme: a.Get(model.SchemeLabel),
		Host:   a.Get(model.AddressLabel),
		Path:   a.Get(pathLabel),
	}
}

// alertmanagerSet contains a set of Alertmanagers discovered via a group of service
// discovery definitions that have a common configuration on how alerts should be sent.
type alertmanagerSet struct {
	cfg    *config.AlertmanagerConfig
	client *http.Client

	mtx        sync.RWMutex
	ams        []alertmanager
	droppedAms []alertmanager
	logger     log.Logger
}

func newAlertmanagerSet(cfg *config.AlertmanagerConfig, logger log.Logger) (*alertmanagerSet, error) {
	client, err := config_util.NewClientFromConfig(cfg.HTTPClientConfig, "alertmanager")
	if err != nil {
		return nil, err
	}
	s := &alertmanagerSet{
		client: client,
		cfg:    cfg,
		logger: logger,
	}
	return s, nil
}

// sync extracts a deduplicated set of Alertmanager endpoints from a list
// of target groups definitions.
func (s *alertmanagerSet) sync(tgs []*targetgroup.Group) {
	allAms := []alertmanager{}
	allDroppedAms := []alertmanager{}

	for _, tg := range tgs {
		ams, droppedAms, err := alertmanagerFromGroup(tg, s.cfg)
		if err != nil {
			level.Error(s.logger).Log("msg", "Creating discovered Alertmanagers failed", "err", err)
			continue
		}
		allAms = append(allAms, ams...)
		allDroppedAms = append(allDroppedAms, droppedAms...)
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()
	// Set new Alertmanagers and deduplicate them along their unique URL.
	s.ams = []alertmanager{}
	s.droppedAms = []alertmanager{}
	s.droppedAms = append(s.droppedAms, allDroppedAms...)
	seen := map[string]struct{}{}

	for _, am := range allAms {
		us := am.url().String()
		if _, ok := seen[us]; ok {
			continue
		}

		// This will initialise the Counters for the AM to 0.
		// s.metrics.sent.WithLabelValues(us)
		// s.metrics.errors.WithLabelValues(us)

		seen[us] = struct{}{}
		s.ams = append(s.ams, am)
	}
}

func postPath(pre string) string {
	return path.Join("/", pre, alertPushEndpoint)
}

// alertmanagersFromGroup extracts a list of alertmanagers from a target group
// and an associated AlertmanagerConfig.
func alertmanagerFromGroup(tg *targetgroup.Group, cfg *config.AlertmanagerConfig) ([]alertmanager, []alertmanager, error) {
	var res []alertmanager
	var droppedAlertManagers []alertmanager

	for _, tlset := range tg.Targets {
		lbls := make([]labels.Label, 0, len(tlset)+2+len(tg.Labels))

		for ln, lv := range tlset {
			lbls = append(lbls, labels.Label{Name: string(ln), Value: string(lv)})
		}
		// Set configured scheme as the initial scheme label for overwrite.
		lbls = append(lbls, labels.Label{Name: model.SchemeLabel, Value: cfg.Scheme})
		lbls = append(lbls, labels.Label{Name: pathLabel, Value: postPath(cfg.PathPrefix)})

		// Combine target labels with target group labels.
		for ln, lv := range tg.Labels {
			if _, ok := tlset[ln]; !ok {
				lbls = append(lbls, labels.Label{Name: string(ln), Value: string(lv)})
			}
		}

		lset := make(model.LabelSet, 0)
		for _, l := range lbls {
			lname := model.LabelName(l.Name)
			lvalue := model.LabelValue(l.Value)
			lset[lname] = lvalue
		}

		relabledSet := relabel.Process(lset, cfg.RelabelConfigs...)
		if relabledSet == nil {
			droppedAlertManagers = append(droppedAlertManagers, alertmanagerLabels{lbls})
			continue
		}
		newLbls := make(labels.Labels, 0)
		for name, val := range relabledSet {
			l := labels.Label{
				Name: string(name), Value: string(val),
			}
			newLbls = append(newLbls, l)
		}

		lb := labels.NewBuilder(newLbls)

		// addPort checks whether we should add a default port to the address.
		// If the address is not valid, we don't append a port either.
		addPort := func(s string) bool {
			// If we can split, a port exists and we don't have to add one.
			if _, _, err := net.SplitHostPort(s); err == nil {
				return false
			}
			// If adding a port makes it valid, the previous error
			// was not due to an invalid address and we can append a port.
			_, _, err := net.SplitHostPort(s + ":1234")
			return err == nil
		}
		addr, ok := lset[model.AddressLabel]
		// If it's an address with no trailing port, infer it based on the used scheme.
		if ok && addPort(string(addr)) {
			// Addresses reaching this point are already wrapped in [] if necessary.
			switch lset[model.SchemeLabel] {
			case "http", "":
				addr = addr + ":80"
			case "https":
				addr = addr + ":443"
			default:
				return nil, nil, fmt.Errorf("invalid scheme: %q", cfg.Scheme)
			}
			lb.Set(model.AddressLabel, string(addr))
		}

		if err := config.CheckTargetAddress(model.LabelValue(addr)); err != nil {
			return nil, nil, err
		}

		// Meta labels are deleted after relabelling. Other internal labels propagate to
		// the target which decides whether they will be part of their label set.
		for _, l := range newLbls {
			if strings.HasPrefix(l.Name, model.MetaLabelPrefix) {
				lb.Del(l.Name)
			}
		}

		res = append(res, alertmanagerLabels{newLbls})
	}
	return res, droppedAlertManagers, nil
}
