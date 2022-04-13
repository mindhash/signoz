package rules

import (
	"context"
	"math"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/pkg/timestamp"
	"github.com/prometheus/prometheus/pkg/value"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/storage"

	opentracing "github.com/opentracing/opentracing-go"
)

type RuleHealth string

const (
	HealthUnknown RuleHealth = "unknown"
	HealthGood    RuleHealth = "ok"
	HealthBad     RuleHealth = "err"
)

// namespace for prom metrics
const namespace = "signoz"

var (
	evalDuration = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Namespace: namespace,
			Name:      "rule_evaluation_duration_seconds",
			Help:      "The duration for a rule to execute.",
		},
	)
	evalFailures = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "rule_evaluation_failures_total",
			Help:      "The total number of rule evaluation failures.",
		},
	)
	evalTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "rule_evaluations_total",
			Help:      "The total number of rule evaluations.",
		},
	)
	iterationDuration = prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace:  namespace,
		Name:       "rule_group_duration_seconds",
		Help:       "The duration of rule group evaluations.",
		Objectives: map[float64]float64{0.01: 0.001, 0.05: 0.005, 0.5: 0.05, 0.90: 0.01, 0.99: 0.001},
	})
	iterationsMissed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "rule_group_iterations_missed_total",
		Help:      "The total number of rule group evaluations missed due to slow rule group evaluation.",
	})
	iterationsScheduled = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "rule_group_iterations_total",
		Help:      "The total number of scheduled rule group evaluations, whether executed or missed.",
	})
	lastDuration = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "rule_group_last_duration_seconds"),
		"The duration of the last rule group evaluation.",
		[]string{"rule_group"},
		nil,
	)
	groupInterval = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "rule_group_interval_seconds"),
		"The interval of a rule group.",
		[]string{"rule_group"},
		nil,
	)
)

func init() {
	prometheus.MustRegister(iterationDuration)
	prometheus.MustRegister(iterationsScheduled)
	prometheus.MustRegister(iterationsMissed)
	prometheus.MustRegister(evalFailures)
	prometheus.MustRegister(evalDuration)
}

// A Rule encapsulates a vector expression which is evaluated at a specified
// interval and acted upon (currently used for alerting).
type Rule interface {
	Name() string
	Labels() labels.Labels
	Eval(context.Context, time.Time, QueryFunc, *url.URL) (Vector, error)
	String() string
	// Query() string
	SetLastError(error)
	LastError() error
	SetHealth(RuleHealth)
	Health() RuleHealth
	SetEvaluationDuration(time.Duration)
	GetEvaluationDuration() time.Duration
	SetEvaluationTimestamp(time.Time)
	GetEvaluationTimestamp() time.Time
}

// Group is a set of rules that have a logical relation.
type Group struct {
	name                 string
	file                 string
	interval             time.Duration
	rules                []Rule
	seriesInPreviousEval []map[string]labels.Labels // One per Rule.
	staleSeries          []labels.Labels
	opts                 *ManagerOptions
	mtx                  sync.Mutex
	evaluationTime       time.Duration
	lastEvaluation       time.Time

	shouldRestore bool

	markStale   bool
	done        chan struct{}
	terminated  chan struct{}
	managerDone chan struct{}

	logger log.Logger
}

type GroupOptions struct {
	Name          string
	File          string
	Interval      time.Duration
	Rules         []Rule
	ShouldRestore bool
	Opts          *ManagerOptions
	done          chan struct{}
}

func groupKey(name, file string) string {
	// TODO: address ; in the group key for existing deployments
	return name + ";" + file
}

// NewGroup makes a new Group with the given name, options, and rules.
func NewGroup(o GroupOptions) *Group {
	return &Group{
		name:                 o.Name,
		file:                 o.File,
		interval:             o.Interval,
		rules:                o.Rules,
		shouldRestore:        o.ShouldRestore,
		opts:                 o.Opts,
		seriesInPreviousEval: make([]map[string]labels.Labels, len(o.Rules)),
		done:                 make(chan struct{}),
		managerDone:          o.done,
		terminated:           make(chan struct{}),
		logger:               log.With(o.Opts.Logger, "group", o.Name),
	}
}

// Name returns the group name.
func (g *Group) Name() string { return g.name }

// Rules returns the group's rules.
func (g *Group) Rules() []Rule { return g.rules }

// Interval returns the group's interval.
func (g *Group) Interval() time.Duration { return g.interval }

func (g *Group) run(ctx context.Context) {
	defer close(g.terminated)

	// Wait an initial amount to have consistently slotted intervals.
	evalTimestamp := g.EvalTimestamp(time.Now().UnixNano()).Add(g.interval)

	select {
	case <-time.After(time.Until(evalTimestamp)):
	case <-g.done:
		return
	}

	ctx = NewQueryOriginContext(ctx, map[string]interface{}{
		"ruleGroup": map[string]string{
			"name": g.Name(),
		},
	})

	iter := func() {
		iterationsScheduled.Inc()

		start := time.Now()
		g.Eval(ctx, evalTimestamp)
		timeSinceStart := time.Since(start)

		iterationDuration.Observe(timeSinceStart.Seconds())
		g.setEvaluationTime(timeSinceStart)
		g.setLastEvaluation(start)
	}

	// The assumption here is that since the ticker was started after having
	// waited for `evalTimestamp` to pass, the ticks will trigger soon
	// after each `evalTimestamp + N * g.interval` occurrence.
	tick := time.NewTicker(g.interval)
	defer tick.Stop()

	// defer cleanup
	defer func() {
		if !g.markStale {
			return
		}
		go func(now time.Time) {
			for _, rule := range g.seriesInPreviousEval {
				for _, r := range rule {
					g.staleSeries = append(g.staleSeries, r)
				}
			}
			// That can be garbage collected at this point.
			g.seriesInPreviousEval = nil

			// Wait for 2 intervals to give the opportunity to renamed rules
			// to insert new series in database. At this point if there is a
			// renamed rule, it should already be started.
			select {
			case <-g.managerDone:
			case <-time.After(2 * g.interval):
				g.cleanupStaleSeries(ctx, now)
			}
		}(time.Now())

	}()

	iter()
	if g.shouldRestore {
		// If we have to restore, we wait for another Eval to finish.
		// The reason behind this is, during first eval (or before it)
		// we might not have enough data scraped, and recording rules would not
		// have updated the latest values, on which some alerts might depend.
		select {
		case <-g.done:
			return
		case <-tick.C:
			missed := (time.Since(evalTimestamp) / g.interval) - 1
			if missed > 0 {
				g.metrics.IterationsMissed.WithLabelValues(GroupKey(g.file, g.name)).Add(float64(missed))
				g.metrics.IterationsScheduled.WithLabelValues(GroupKey(g.file, g.name)).Add(float64(missed))
			}
			evalTimestamp = evalTimestamp.Add((missed + 1) * g.interval)
			iter()
		}

		g.RestoreForState(time.Now())
		g.shouldRestore = false
	}

	// let the group iterate and run
	for {
		select {
		case <-g.done:
			return
		default:
			select {
			case <-g.done:
				return
			case <-tick.C:
				missed := (time.Since(evalTimestamp) / g.interval) - 1
				if missed > 0 {
					g.metrics.IterationsMissed.WithLabelValues(GroupKey(g.file, g.name)).Add(float64(missed))
					g.metrics.IterationsScheduled.WithLabelValues(GroupKey(g.file, g.name)).Add(float64(missed))
				}
				evalTimestamp = evalTimestamp.Add((missed + 1) * g.interval)
				iter()
			}
		}
	}
}

func (g *Group) stop() {
	close(g.done)
	<-g.terminated
}

func (g *Group) hash() uint64 {
	l := labels.New(
		labels.Label{Name: "name", Value: g.name},
	)
	return l.Hash()
}

// AlertingRules returns the list of the group's alerting rules.
func (g *Group) AlertingRules() []*AlertingRule {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	var alerts []*AlertingRule
	for _, rule := range g.rules {
		if alertingRule, ok := rule.(*AlertingRule); ok {
			alerts = append(alerts, alertingRule)
		}
	}
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].State() > alerts[j].State() ||
			(alerts[i].State() == alerts[j].State() &&
				alerts[i].Name() < alerts[j].Name())
	})
	return alerts
}

// HasAlertingRules returns true if the group contains at least one AlertingRule.
func (g *Group) HasAlertingRules() bool {
	g.mtx.Lock()
	defer g.mtx.Unlock()

	for _, rule := range g.rules {
		if _, ok := rule.(*AlertingRule); ok {
			return true
		}
	}
	return false
}

// GetEvaluationTime returns the time in seconds it took to evaluate the rule group.
func (g *Group) GetEvaluationTime() time.Duration {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.evaluationTime
}

// setEvaluationTime sets the time in seconds the last evaluation took.
func (g *Group) setEvaluationTime(dur time.Duration) {
	g.metrics.GroupLastDuration.WithLabelValues(GroupKey(g.file, g.name)).Set(dur.Seconds())

	g.mtx.Lock()
	defer g.mtx.Unlock()
	g.evaluationTime = dur
}

// GetLastEvaluation returns the time the last evaluation of the rule group took place.
func (g *Group) GetLastEvaluation() time.Time {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.lastEvaluation
}

// setLastEvaluation updates evaluationTimestamp to the timestamp of when the rule group was last evaluated.
func (g *Group) setLastEvaluation(ts time.Time) {
	g.metrics.GroupLastEvalTime.WithLabelValues(GroupKey(g.file, g.name)).Set(float64(ts.UnixNano()) / 1e9)

	g.mtx.Lock()
	defer g.mtx.Unlock()
	g.lastEvaluation = ts
}

// EvalTimestamp returns the immediately preceding consistently slotted evaluation time.
func (g *Group) EvalTimestamp(startTime int64) time.Time {
	var (
		offset = int64(g.hash() % uint64(g.interval))
		adjNow = startTime - offset
		base   = adjNow - (adjNow % int64(g.interval))
	)

	return time.Unix(0, base+offset).UTC()
}

func nameAndLabels(rule Rule) string {
	return rule.Name() + rule.Labels().String()
}

// CopyState copies the alerting rule and staleness related state from the given group.
//
// Rules are matched based on their name and labels. If there are duplicates, the
// first is matched with the first, second with the second etc.
func (g *Group) CopyState(from *Group) {
	g.evaluationTime = from.evaluationTime
	g.lastEvaluation = from.lastEvaluation

	ruleMap := make(map[string][]int, len(from.rules))

	for fi, fromRule := range from.rules {
		nameAndLabels := nameAndLabels(fromRule)
		l := ruleMap[nameAndLabels]
		ruleMap[nameAndLabels] = append(l, fi)
	}

	for i, rule := range g.rules {
		nameAndLabels := nameAndLabels(rule)
		indexes := ruleMap[nameAndLabels]
		if len(indexes) == 0 {
			continue
		}
		fi := indexes[0]
		g.seriesInPreviousEval[i] = from.seriesInPreviousEval[fi]
		ruleMap[nameAndLabels] = indexes[1:]

		ar, ok := rule.(*AlertingRule)
		if !ok {
			continue
		}
		far, ok := from.rules[fi].(*AlertingRule)
		if !ok {
			continue
		}

		for fp, a := range far.active {
			ar.active[fp] = a
		}
	}

	// Handle deleted and unmatched duplicate rules.
	g.staleSeries = from.staleSeries
	for fi, fromRule := range from.rules {
		nameAndLabels := nameAndLabels(fromRule)
		l := ruleMap[nameAndLabels]
		if len(l) != 0 {
			for _, series := range from.seriesInPreviousEval[fi] {
				g.staleSeries = append(g.staleSeries, series)
			}
		}
	}
}

// Eval runs a single evaluation cycle in which all rules are evaluated sequentially.
func (g *Group) Eval(ctx context.Context, ts time.Time) {
	var samplesTotal float64
	for i, rule := range g.rules {
		select {
		case <-g.done:
			return
		default:
		}

		func(i int, rule Rule) {
			sp, ctx := opentracing.StartSpanFromContext(ctx, "rule")
			sp.SetTag("name", rule.Name())
			defer func(t time.Time) {
				sp.Finish()

				since := time.Since(t)
				g.metrics.EvalDuration.Observe(since.Seconds())
				rule.SetEvaluationDuration(since)
				rule.SetEvaluationTimestamp(t)
			}(time.Now())

			g.metrics.EvalTotal.WithLabelValues(GroupKey(g.File(), g.Name())).Inc()

			vector, err := rule.Eval(ctx, ts, g.opts.QueryFunc, g.opts.ExternalURL)
			if err != nil {
				rule.SetHealth(HealthBad)
				rule.SetLastError(err)
				g.metrics.EvalFailures.WithLabelValues(GroupKey(g.File(), g.Name())).Inc()

				// Canceled queries are intentional termination of queries. This normally
				// happens on shutdown and thus we skip logging of any errors here.
				if _, ok := err.(promql.ErrQueryCanceled); !ok {
					level.Warn(g.logger).Log("msg", "Evaluating rule failed", "rule", rule, "err", err)
				}
				return
			}
			samplesTotal += float64(len(vector))

			if ar, ok := rule.(*AlertingRule); ok {
				ar.sendAlerts(ctx, ts, g.opts.ResendDelay, g.interval, g.opts.NotifyFunc)
			}
			var (
				numOutOfOrder = 0
				numDuplicates = 0
			)

			app := g.opts.Appendable.Appender(ctx)
			seriesReturned := make(map[string]labels.Labels, len(g.seriesInPreviousEval[i]))
			defer func() {
				if err := app.Commit(); err != nil {
					rule.SetHealth(HealthBad)
					rule.SetLastError(err)
					g.metrics.EvalFailures.WithLabelValues(GroupKey(g.File(), g.Name())).Inc()

					level.Warn(g.logger).Log("msg", "Rule sample appending failed", "err", err)
					return
				}
				g.seriesInPreviousEval[i] = seriesReturned
			}()

			for _, s := range vector {
				if _, err := app.Append(0, s.Metric, s.T, s.V); err != nil {
					rule.SetHealth(HealthBad)
					rule.SetLastError(err)

					switch errors.Cause(err) {
					case storage.ErrOutOfOrderSample:
						numOutOfOrder++
						level.Debug(g.logger).Log("msg", "Rule evaluation result discarded", "err", err, "sample", s)
					case storage.ErrDuplicateSampleForTimestamp:
						numDuplicates++
						level.Debug(g.logger).Log("msg", "Rule evaluation result discarded", "err", err, "sample", s)
					default:
						level.Warn(g.logger).Log("msg", "Rule evaluation result discarded", "err", err, "sample", s)
					}
				} else {
					seriesReturned[s.Metric.String()] = s.Metric
				}
			}
			if numOutOfOrder > 0 {
				level.Warn(g.logger).Log("msg", "Error on ingesting out-of-order result from rule evaluation", "numDropped", numOutOfOrder)
			}
			if numDuplicates > 0 {
				level.Warn(g.logger).Log("msg", "Error on ingesting results from rule evaluation with different value but same timestamp", "numDropped", numDuplicates)
			}

			for metric, lset := range g.seriesInPreviousEval[i] {
				if _, ok := seriesReturned[metric]; !ok {
					// Series no longer exposed, mark it stale.
					_, err = app.Append(0, lset, timestamp.FromTime(ts), math.Float64frombits(value.StaleNaN))
					switch errors.Cause(err) {
					case nil:
					case storage.ErrOutOfOrderSample, storage.ErrDuplicateSampleForTimestamp:
						// Do not count these in logging, as this is expected if series
						// is exposed from a different rule.
					default:
						level.Warn(g.logger).Log("msg", "Adding stale sample failed", "sample", metric, "err", err)
					}
				}
			}
		}(i, rule)
	}
	if g.metrics != nil {
		g.metrics.GroupSamples.WithLabelValues(GroupKey(g.File(), g.Name())).Set(samplesTotal)
	}
	g.cleanupStaleSeries(ctx, ts)
}

// TODO(amol): find out what is restore for state
func (g *Group) RestoreForState(ts time.Time) {

}

// The Manager manages recording and alerting rules.
type Manager struct {
	opts     *ManagerOptions
	groups   map[string]*Group
	mtx      sync.RWMutex
	block    chan struct{}
	restored bool

	logger log.Logger
}

// Appendable returns an Appender.
type Appendable interface {
	Appender() (storage.Appender, error)
}

// NotifyFunc sends notifications about a set of alerts generated by the given expression.
type NotifyFunc func(ctx context.Context, expr string, alerts ...*Alert)

// ManagerOptions bundles options for the Manager.
type ManagerOptions struct {
	ExternalURL     *url.URL
	QueryFunc       QueryFunc
	NotifyFunc      NotifyFunc
	Context         context.Context
	Appendable      Appendable
	TSDB            storage.Storage
	Logger          log.Logger
	Registerer      prometheus.Registerer
	OutageTolerance time.Duration
	ForGracePeriod  time.Duration
	ResendDelay     time.Duration
}

// NewManager returns an implementation of Manager, ready to be started
// by calling the Run method.
func NewManager(o *ManagerOptions) *Manager {
	m := &Manager{
		groups: map[string]*Group{},
		opts:   o,
		block:  make(chan struct{}),
		logger: o.Logger,
	}
	if o.Registerer != nil {
		o.Registerer.MustRegister(m)
	}
	return m
}

// Run starts processing of the rule manager.
func (m *Manager) Run() {
	close(m.block)
}

// Stop the rule manager's rule evaluation cycles.
func (m *Manager) Stop() {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	level.Info(m.logger).Log("msg", "Stopping rule manager...")

	for _, eg := range m.groups {
		eg.stop()
	}

	level.Info(m.logger).Log("msg", "Rule manager stopped")
}

// Update the rule manager's state as the config requires. If
// loading the new rules failed the old rule set is restored.
func (m *Manager) Update(interval time.Duration, files []string) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	groups, errs := m.LoadGroups(interval, files...)
	if errs != nil {
		for _, e := range errs {
			level.Error(m.logger).Log("msg", "loading groups failed", "err", e)
		}
		return errors.New("error loading rules, previous rule set restored")
	}
	m.restored = true

	var wg sync.WaitGroup

	for _, newg := range groups {
		wg.Add(1)

		// If there is an old group with the same identifier, stop it and wait for
		// it to finish the current iteration. Then copy it into the new group.
		gn := GroupKey(newg.name, newg.file)
		oldg, ok := m.groups[gn]
		delete(m.groups, gn)

		go func(newg *Group) {
			if ok {
				oldg.stop()
				newg.CopyState(oldg)
			}
			go func() {
				// Wait with starting evaluation until the rule manager
				// is told to run. This is necessary to avoid running
				// queries against a bootstrapping storage.
				<-m.block
				newg.run(m.opts.Context)
			}()
			wg.Done()
		}(newg)
	}

	// Stop remaining old groups.
	for _, oldg := range m.groups {
		oldg.stop()
	}

	wg.Wait()
	m.groups = groups

	return nil
}
