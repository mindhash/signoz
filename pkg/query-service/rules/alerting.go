package rules

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/influxdata/promql/v2"

	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/pkg/timestamp"
	"github.com/prometheus/prometheus/template"
)

const (
	// AlertMetricName is the metric name for synthetic alert timeseries.
	alertMetricName = "ALERTS"

	// AlertForStateMetricName is the metric name for 'for' state of alert.
	alertForStateMetricName = "ALERTS_FOR_STATE"

	// AlertNameLabel is the label name indicating the name of an alert.
	alertNameLabel = "alertname"

	// AlertStateLabel is the label name indicating the state of an alert.
	alertStateLabel = "alertstate"
)

// AlertState denotes the state of an active alert.
type AlertState int

const (
	StateInactive AlertState = iota
	StatePending
	StateFiring
)

func (s AlertState) String() string {
	switch s {
	case StateInactive:
		return "inactive"
	case StatePending:
		return "pending"
	case StateFiring:
		return "firing"
	}
	panic(errors.Errorf("unknown alert state: %d", s))
}

type Alert struct {
	State AlertState

	Labels      labels.Labels
	Annotations labels.Labels

	Value      float64
	ActiveAt   time.Time
	FiredAt    time.Time
	ResolvedAt time.Time
	LastSentAt time.Time
	ValidUntil time.Time
}

type AlertingRule struct {
	name string
	// rule string json
	// ruleConditions[]

	query string

	holdDuration time.Duration
	labels       labels.Labels
	annotations  labels.Labels

	// external labels and url from global config
	externalLabels map[string]string
	externalURL    string

	// true if old state has been restored. We start persisting samples for ALERT_FOR_STATE
	// only after the restoration.
	restored bool

	mtx                 sync.Mutex
	evaluationDuration  time.Duration
	evaluationTimestamp time.Time

	health RuleHealth

	lastError error

	// map of active alerts
	active map[uint64]*Alert

	logger log.Logger
}

func NewAlertingRule(
	name string, hold time.Duration, labels, annotations, externalLabels labels.Labels,
	externalURL string, restored bool, logger log.Logger,
) *AlertingRule {
	el := make(map[string]string, len(externalLabels))
	for _, lbl := range externalLabels {
		el[lbl.Name] = lbl.Value
	}

	return &AlertingRule{
		name: name,
		// TODO: rule conditions
		holdDuration:   hold,
		labels:         labels,
		annotations:    annotations,
		externalLabels: el,
		externalURL:    externalURL,
		health:         HealthUnknown,
		active:         map[uint64]*Alert{},
		logger:         logger,
		restored:       restored,
	}
}

func (r *AlertingRule) Name() string {
	return r.name
}

func (r *AlertingRule) SetLastError(err error) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.lastError = err
}

func (r *AlertingRule) LastError() error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	return r.lastError
}

func (r *AlertingRule) SetHealth(health RuleHealth) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.health = health
}

func (r *AlertingRule) Health() RuleHealth {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	return r.health
}

func (r *AlertingRule) SetRestored(restored bool) {
	r.restored = restored
}

// SetEvaluationDuration updates evaluationDuration to the duration it took to evaluate the rule on its last evaluation.
func (r *AlertingRule) SetEvaluationDuration(dur time.Duration) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.evaluationDuration = dur
}

func (r *AlertingRule) HoldDuration() time.Duration {
	return r.holdDuration
}

// Labels returns the labels of the alerting rule.
func (r *AlertingRule) Labels() labels.Labels {
	return r.labels
}

// Annotations returns the annotations of the alerting rule.
func (r *AlertingRule) Annotations() labels.Labels {
	return r.annotations
}

func (r *AlertingRule) sample(alert *Alert, ts time.Time) promql.Sample {
	lb := labels.NewBuilder(r.labels)

	for _, l := range alert.Labels {
		lb.Set(l.Name, l.Value)
	}

	lb.Set(labels.MetricName, alertMetricName)
	lb.Set(labels.AlertName, r.name)
	lb.Set(alertStateLabel, alert.State.String())

	s := Sample{
		Metric: lb.Labels(),
		Point:  Point{T: timestamp.FromTime(ts), V: 1},
	}
	return s
}

// forStateSample returns the sample for ALERTS_FOR_STATE.
func (r *AlertingRule) forStateSample(alert *Alert, ts time.Time, v float64) promql.Sample {
	lb := labels.NewBuilder(r.labels)

	for _, l := range alert.Labels {
		lb.Set(l.Name, l.Value)
	}

	lb.Set(labels.MetricName, alertForStateMetricName)
	lb.Set(labels.AlertName, r.name)

	s := Sample{
		Metric: lb.Labels(),
		Point:  Point{T: timestamp.FromTime(ts), V: v},
	}
	return s
}

// GetEvaluationDuration returns the time in seconds it took to evaluate the alerting rule.
func (r *AlertingRule) GetEvaluationDuration() time.Duration {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	return r.evaluationDuration
}

// SetEvaluationTimestamp updates evaluationTimestamp to the timestamp of when the rule was last evaluated.
func (r *AlertingRule) SetEvaluationTimestamp(ts time.Time) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.evaluationTimestamp = ts
}

// GetEvaluationTimestamp returns the time the evaluation took place.
func (r *AlertingRule) GetEvaluationTimestamp() time.Time {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	return r.evaluationTimestamp
}

const resolvedRetention = 15 * time.Minute

// State returns the maximum state of alert instances for this rule.
// StateFiring > StatePending > StateInactive
func (r *AlertingRule) State() AlertState {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	maxState := StateInactive
	for _, a := range r.active {
		if a.State > maxState {
			maxState = a.State
		}
	}
	return maxState
}

func (r *AlertingRule) currentAlerts() []*Alert {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	alerts := make([]*Alert, 0, len(r.active))

	for _, a := range r.active {
		anew := *a
		alerts = append(alerts, &anew)
	}
	return alerts
}

func (r *AlertingRule) ActiveAlerts() []*Alert {
	var res []*Alert
	for _, a := range r.currentAlerts() {
		if a.ResolvedAt.IsZero() {
			res = append(res, a)
		}
	}
	return res
}

// ForEachActiveAlert runs the given function on each alert.
// This should be used when you want to use the actual alerts from the AlertingRule
// and not on its copy.
// If you want to run on a copy of alerts then don't use this, get the alerts from 'ActiveAlerts()'.
func (r *AlertingRule) ForEachActiveAlert(f func(*Alert)) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	for _, a := range r.active {
		f(a)
	}
}

func (a *Alert) needsSending(ts time.Time, resendDelay time.Duration) bool {
	if a.State == StatePending {
		return false
	}

	// if an alert has been resolved since the last send, resend it
	if a.ResolvedAt.After(a.LastSentAt) {
		return true
	}

	return a.LastSentAt.Add(resendDelay).Before(ts)
}

func (r *AlertingRule) sendAlerts(ctx context.Context, ts time.Time, resendDelay time.Duration, interval time.Duration, notifyFunc NotifyFunc) {
	alerts := []*Alert{}
	r.ForEachActiveAlert(func(alert *Alert) {
		if alert.needsSending(ts, resendDelay) {
			alert.LastSentAt = ts
			// Allow for two Eval or Alertmanager send failures.
			delta := resendDelay
			if interval > resendDelay {
				delta = interval
			}
			alert.ValidUntil = ts.Add(4 * delta)
			anew := *alert
			alerts = append(alerts, &anew)
		}
	})
	// todo: handle query
	notifyFunc(ctx, r.query, alerts...)
}

func (r *AlertingRule) Eval(ctx context.Context, ts time.Time, query QueryFunc, externalURL *url.URL) (Vector, error) {
	res, err := query(ctx, r.query, ts)
	if err != nil {
		r.SetHealth(HealthBad)
		r.SetLastError(err)
		return nil, err
	}

	r.mtx.Lock()
	defer r.mtx.Unlock()

	resultFPs := map[uint64]struct{}{}
	var vec Vector
	var alerts = make(map[uint64]*Alert, len(res))

	for _, smpl := range res {
		l := make(map[string]string, len(smpl.Metric))
		for _, lbl := range smpl.Metric {
			l[lbl.Name] = lbl.Value
		}

		tmplData := template.AlertTemplateData(l, r.externalLabels, r.externalURL, smpl.V)
		defs := []string{
			"{{$labels := .Labels}}",
			"{{$externalLabels := .ExternalLabels}}",
			"{{$externalURL := .ExternalURL}}",
			"{{$value := .Value}}",
		}

		expand := func(text string) string {
			tmpl := template.NewTemplateExpander(
				ctx,
				strings.Join(append(defs, text), ""),
				"__alert_"+r.Name(),
				tmplData,
				model.Time(timestamp.FromTime(ts)),
				template.QueryFunc(query),
				externalURL,
			)
			result, err := tmpl.Expand()
			if err != nil {
				result = fmt.Sprintf("<error expanding template: %s>", err)
				level.Warn(r.logger).Log("msg", "Expanding alert template failed", "err", err, "data", tmplData)
			}
			return result
		}

		lb := labels.NewBuilder(smpl.Metric).Del(labels.MetricName)

		for _, l := range r.labels {
			lb.Set(l.Name, expand(l.Value))
		}
		lb.Set(labels.AlertName, r.Name())

		annotations := make(labels.Labels, 0, len(r.annotations))
		for _, a := range r.annotations {
			annotations = append(annotations, labels.Label{Name: a.Name, Value: expand(a.Value)})
		}

		lbs := lb.Labels()
		h := lbs.Hash()
		resultFPs[h] = struct{}{}

		if _, ok := alerts[h]; ok {
			err = fmt.Errorf("vector contains metrics with the same labelset after applying alert labels")
			// We have already acquired the lock above hence using SetHealth and
			// SetLastError will deadlock.
			r.health = HealthBad
			r.lastError = err
			return nil, err
		}

		alerts[h] = &Alert{
			Labels:      lbs,
			Annotations: annotations,
			ActiveAt:    ts,
			State:       StatePending,
			Value:       smpl.V,
		}
	}

	// alerts[h] is ready, add or update active list now
	for h, a := range alerts {
		// Check whether we already have alerting state for the identifying label set.
		// Update the last value and annotations if so, create a new alert entry otherwise.
		if alert, ok := r.active[h]; ok && alert.State != StateInactive {
			alert.Value = a.Value
			alert.Annotations = a.Annotations
			continue
		}

		r.active[h] = a

	}

	// Check if any pending alerts should be removed or fire now. Write out alert timeseries.
	for fp, a := range r.active {
		if _, ok := resultFPs[fp]; !ok {
			// If the alert was previously firing, keep it around for a given
			// retention time so it is reported as resolved to the AlertManager.
			if a.State == StatePending || (!a.ResolvedAt.IsZero() && ts.Sub(a.ResolvedAt) > resolvedRetention) {
				delete(r.active, fp)
			}
			if a.State != StateInactive {
				a.State = StateInactive
				a.ResolvedAt = ts
			}
			continue
		}

		if a.State == StatePending && ts.Sub(a.ActiveAt) >= r.holdDuration {
			a.State = StateFiring
			a.FiredAt = ts
		}

		if r.restored {
			vec = append(vec, r.sample(a, ts))
			vec = append(vec, r.forStateSample(a, ts, float64(a.ActiveAt.Unix())))
		}

	}
	r.health = HealthGood
	r.lastError = err
	return vec, nil

}
