package rules

import (
	"encoding/json"
	"github.com/pkg/errors"
	"go.signoz.io/query-service/model"
	"go.signoz.io/query-service/utils/labels"
	"time"
)

// how long before re-sending the alert
const resolvedRetention = 15 * time.Minute

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

type RuleType string

const (
	RuleTypeThreshold = "threshold_rule"
	RuleTypeProm      = "promql_rule"
)

type RuleHealth string

const (
	HealthUnknown RuleHealth = "unknown"
	HealthGood    RuleHealth = "ok"
	HealthBad     RuleHealth = "err"
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

	Labels      labels.BaseLabels
	Annotations labels.BaseLabels

	Value      float64
	ActiveAt   time.Time
	FiredAt    time.Time
	ResolvedAt time.Time
	LastSentAt time.Time
	ValidUntil time.Time
}

// todo(amol): need to review this with ankit
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

type NamedAlert struct {
	Name string
	*Alert
}

type CompareOp string

const (
	ValueIsAbove CompareOp = "0"
	ValueIsBelow CompareOp = "1"
	ValueIsEq    CompareOp = "2"
	ValueIsNotEq CompareOp = "3"
)

type MatchType string

const (
	AllTheTimes MatchType = "0"
	AtleastOnce MatchType = "1"
	OnAverage   MatchType = "2"
	InTotal     MatchType = "3"
)

type RuleCondition struct {
	CompositeMetricQuery *model.CompositeMetricQuery `json:"compositeMetricQuery,omitempty" yaml:"compositeMetricQuery,omitempty"`
	CompareOp            CompareOp                   `yaml:"op,omitempty" json:"op,omitempty"`
	Target               *float64                    `yaml:"target,omitempty" json:"target,omitempty"`
	MatchType            `json:"matchType,omitempty"`
}

func (rc *RuleCondition) String() string {
	if rc == nil {
		return ""
	}
	data, _ := json.Marshal(*rc)
	return string(data)
}

type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(tmp)

		return nil
	default:
		return errors.New("invalid duration")
	}
}