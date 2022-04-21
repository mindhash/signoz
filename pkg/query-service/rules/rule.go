package rules

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"go.signoz.io/query-service/utils/timestamp"
	yaml "gopkg.in/yaml.v2"
)

// PostableRule is used to create alerting rule from HTTP api
type PostableRule struct {
	Alert       string            `yaml:"alert,omitempty"`
	Expr        string            `yaml:"expr"`
	For         time.Duration     `yaml:"for,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

func ParsePostableRule(content []byte) (*PostableRule, []error) {
	rule := PostableRule{}
	if err := yaml.UnmarshalStrict(content, &rule); err != nil {
		return nil, []error{err}
	}
	if errs := rule.Validate(); len(errs) > 0 {
		return nil, errs
	}
	return &rule, []error{}
}

func (r *PostableRule) Validate() (errs []error) {
	for k, v := range r.Labels {
		if !model.LabelName(k).IsValid() {
			errs = append(errs, errors.Errorf("invalid label name: %s", k))
		}

		if !model.LabelValue(v).IsValid() {
			errs = append(errs, errors.Errorf("invalid label value: %s", v))
		}
	}

	for k := range r.Annotations {
		if !model.LabelName(k).IsValid() {
			errs = append(errs, errors.Errorf("invalid annotation name: %s", k))
		}
	}

	errs = append(errs, testTemplateParsing(r)...)
	return errs
}

func testTemplateParsing(rl *PostableRule) (errs []error) {
	if rl.Alert == "" {
		// Not an alerting rule.
		return errs
	}

	// Trying to parse templates.
	tmplData := AlertTemplateData(make(map[string]string), 0)
	defs := "{{$labels := .Labels}}{{$value := .Value}}"
	parseTest := func(text string) error {
		tmpl := NewTemplateExpander(
			context.TODO(),
			defs+text,
			"__alert_"+rl.Alert,
			tmplData,
			model.Time(timestamp.FromTime(time.Now())),
			nil,
			nil,
		)
		return tmpl.ParseTest()
	}

	// Parsing Labels.
	for _, val := range rl.Labels {
		err := parseTest(val)
		if err != nil {
			errs = append(errs, fmt.Errorf("msg=%s", err.Error()))
		}
	}

	// Parsing Annotations.
	for _, val := range rl.Annotations {
		err := parseTest(val)
		if err != nil {
			errs = append(errs, fmt.Errorf("msg=%s", err.Error()))
		}
	}

	return errs
}
