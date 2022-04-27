package rules

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"go.signoz.io/query-service/utils/labels"
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
	evaluationDuration   time.Duration
	evaluationTime       time.Duration
	lastEvaluation       time.Time

	markStale   bool
	done        chan struct{}
	terminated  chan struct{}
	managerDone chan struct{}

	logger log.Logger
}

func groupKey(name, file string) string {
	return name + ";" + file
}

// NewGroup makes a new Group with the given name, options, and rules.
func NewGroup(name, file string, interval time.Duration, rules []Rule, opts *ManagerOptions) *Group {

	return &Group{
		name:                 name,
		file:                 file,
		interval:             interval,
		rules:                rules,
		opts:                 opts,
		seriesInPreviousEval: make([]map[string]labels.Labels, len(rules)),
		done:                 make(chan struct{}),
		terminated:           make(chan struct{}),
		logger:               log.With(opts.Logger, "group", name),
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

		start := time.Now()
		g.Eval(ctx, evalTimestamp)
		timeSinceStart := time.Since(start)

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

		}(time.Now())

	}()

	iter()

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

// GetEvaluationDuration returns the time in seconds it took to evaluate the rule group.
func (g *Group) GetEvaluationDuration() time.Duration {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.evaluationDuration
}

// SetEvaluationDuration sets the time in seconds the last evaluation took.
func (g *Group) SetEvaluationDuration(dur time.Duration) {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	g.evaluationDuration = dur
}

// GetEvaluationTime returns the time in seconds it took to evaluate the rule group.
func (g *Group) GetEvaluationTime() time.Duration {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.evaluationTime
}

// setEvaluationTime sets the time in seconds the last evaluation took.
func (g *Group) setEvaluationTime(dur time.Duration) {
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
		if rule == nil {
			continue
		}
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
				rule.SetEvaluationDuration(since)
				rule.SetEvaluationTimestamp(t)
			}(time.Now())

			vector, err := rule.Eval(ctx, ts, g.opts.QueryFunc, g.opts.ExternalURL)
			if err != nil {
				rule.SetHealth(HealthBad)
				rule.SetLastError(err)

				// Canceled queries are intentional termination of queries. This normally
				// happens on shutdown and thus we skip logging of any errors here.
				//! if _, ok := err.(promql.ErrQueryCanceled); !ok {
				//	level.Warn(g.logger).Log("msg", "Evaluating rule failed", "rule", rule, "err", err)
				//}
				return
			}
			samplesTotal += float64(len(vector))

			if ar, ok := rule.(*AlertingRule); ok {
				ar.sendAlerts(ctx, ts, g.opts.ResendDelay, g.interval, g.opts.NotifyFunc)
			}

			seriesReturned := make(map[string]labels.Labels, len(g.seriesInPreviousEval[i]))

			defer func() {
				g.seriesInPreviousEval[i] = seriesReturned
			}()

			for _, s := range vector {
				seriesReturned[s.Metric.String()] = s.Metric
			}

		}(i, rule)
	}
}

// The Manager manages recording and alerting rules.
type Manager struct {
	opts   *ManagerOptions
	groups map[string]*Group
	mtx    sync.RWMutex
	block  chan struct{}

	logger log.Logger
}

// NotifyFunc sends notifications about a set of alerts generated by the given expression.
type NotifyFunc func(ctx context.Context, expr string, alerts ...*Alert)

// ManagerOptions bundles options for the Manager.
type ManagerOptions struct {
	ExternalURL *url.URL
	QueryFunc   QueryFunc
	NotifyFunc  NotifyFunc
	Context     context.Context
	Logger      log.Logger
	Registerer  prometheus.Registerer
	ResendDelay time.Duration
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

func (m *Manager) EditGroup(interval time.Duration, rule string, groupName string) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	// m.restored = true

	var groups map[string]*Group
	var errs []error

	groups, errs = m.LoadGroup(interval, rule, groupName)

	if errs != nil {
		for _, e := range errs {
			level.Error(m.logger).Log("msg", "loading groups failed", "err", e)
		}
		return errors.New("error loading rules, previous rule set restored")
	}

	var wg sync.WaitGroup

	for _, newg := range groups {
		wg.Add(1)

		// If there is an old group with the same identifier, stop it and wait for
		// it to finish the current iteration. Then copy it into the new group.
		gn := groupKey(newg.name, newg.file)
		oldg, ok := m.groups[gn]
		if !ok {
			return errors.New("rule not found")
		}

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

		m.groups[gn] = newg

	}

	// // Stop remaining old groups.
	// for _, oldg := range m.groups {
	// 	oldg.stop()
	// }

	wg.Wait()

	return nil
}

func (m *Manager) DeleteGroup(interval time.Duration, rule string, groupName string) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	// m.restored = true

	filename := "webAppEditor"

	gn := groupKey(groupName, filename)
	oldg, ok := m.groups[gn]

	var wg sync.WaitGroup
	wg.Add(1)
	go func(newg *Group) {
		if ok {
			oldg.stop()
			delete(m.groups, gn)
		}
		defer wg.Done()
	}(oldg)

	wg.Wait()

	return nil
}

func (m *Manager) AddGroup(interval time.Duration, rule string, groupName string) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	// m.restored = true

	var groups map[string]*Group
	var errs []error

	groups, errs = m.LoadGroup(interval, rule, groupName)

	if errs != nil {
		for _, e := range errs {
			level.Error(m.logger).Log("msg", "loading groups failed", "err", e)
		}
		return errors.New("error loading rules, previous rule set restored")
	}

	var wg sync.WaitGroup

	for _, newg := range groups {
		wg.Add(1)

		// If there is an old group with the same identifier, stop it and wait for
		// it to finish the current iteration. Then copy it into the new group.
		gn := groupKey(newg.name, newg.file)
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

		if !ok {
			m.groups[gn] = newg
		}
	}

	wg.Wait()

	return nil
}

// LoadGroup loads a given rule in rule manager
func (m *Manager) LoadGroup(interval time.Duration, rule string, groupName string) (map[string]*Group, []error) {

	// shouldRestore := !m.restored
	filename := "webAppEditor"
	r, errs := ParsePostableRule([]byte(rule))

	if len(errs) > 0 {
		return nil, errs
	}

	if interval == 0 {
		return nil, []error{fmt.Errorf("group interval can not be zero")}
	}

	rules := make([]Rule, 0)
	if r.Alert != "" {
		rules = append(rules, NewAlertingRule(
			r.Alert,
			r.Query,
			time.Duration(r.For),
			labels.FromMap(r.Labels),
			labels.FromMap(r.Annotations),
			log.With(m.logger, "alert", r.Alert),
		))
	}

	groups := make(map[string]*Group)
	groups[groupKey(groupName, filename)] = NewGroup(groupName, filename, interval, rules, m.opts)

	return groups, nil
}

// RuleGroups returns the list of manager's rule groups.
func (m *Manager) RuleGroups() []*Group {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	rgs := make([]*Group, 0, len(m.groups))
	for _, g := range m.groups {
		rgs = append(rgs, g)
	}

	sort.Slice(rgs, func(i, j int) bool {
		return rgs[i].file < rgs[j].file && rgs[i].name < rgs[j].name
	})

	return rgs
}

// RuleGroups returns the list of manager's rule groups.
func (m *Manager) RuleGroupsWithoutLock() []*Group {

	rgs := make([]*Group, 0, len(m.groups))
	for _, g := range m.groups {
		rgs = append(rgs, g)
	}

	sort.Slice(rgs, func(i, j int) bool {
		return rgs[i].file < rgs[j].file && rgs[i].name < rgs[j].name
	})

	return rgs
}

// Rules returns the list of the manager's rules.
func (m *Manager) Rules() []Rule {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	var rules []Rule
	for _, g := range m.groups {
		rules = append(rules, g.rules...)
	}

	return rules
}

// AlertingRules returns the list of the manager's alerting rules.
func (m *Manager) AlertingRules() []*AlertingRule {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	alerts := []*AlertingRule{}
	for _, rule := range m.Rules() {
		if alertingRule, ok := rule.(*AlertingRule); ok {
			alerts = append(alerts, alertingRule)
		}
	}
	return alerts
}

// Describe implements prometheus.Collector.
func (m *Manager) Describe(ch chan<- *prometheus.Desc) {
	ch <- lastDuration
	ch <- groupInterval
}

// Collect implements prometheus.Collector.
func (m *Manager) Collect(ch chan<- prometheus.Metric) {
	for _, g := range m.RuleGroups() {
		ch <- prometheus.MustNewConstMetric(lastDuration,
			prometheus.GaugeValue,
			g.GetEvaluationDuration().Seconds(),
			groupKey(g.file, g.name))
	}
	for _, g := range m.RuleGroups() {
		ch <- prometheus.MustNewConstMetric(groupInterval,
			prometheus.GaugeValue,
			g.interval.Seconds(),
			groupKey(g.file, g.name))
	}
}
