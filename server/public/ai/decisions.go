// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package ai

import (
	"errors"
	"fmt"
)

// Decisions: typed answers about a text from a decision model, instead of generated text. A request carries the
// evidence (Input) once and a list of questions; each answer gives probabilities the caller can act on.
//
// Two wire dialects exist; the connection says which one its endpoint speaks:
//   - DecisionsDialectOpenAI: OpenAI's POST /v1/decisions (questions and answers as lists; predicate, choice, score);
//   - DecisionsDialectSystemOne: POST /v1/systemone, the native form of Jev and Laya (state, questions and answers
//     keyed by name; "noul" for a yes/no probability, criteria for choices and score levels).
//
// Callers always use these types; the provider maps them to the dialect.

// DecisionsDialect is the wire format of a decisions endpoint.
type DecisionsDialect string

const (
	DecisionsDialectOpenAI    DecisionsDialect = "openai"
	DecisionsDialectSystemOne DecisionsDialect = "systemone"
)

// IsValid tells whether d is a dialect the providers speak.
func (d DecisionsDialect) IsValid() bool {
	return d == DecisionsDialectOpenAI || d == DecisionsDialectSystemOne
}

// DecisionType is the kind of a question.
type DecisionType string

const (
	// DecisionPredicate is a yes/no condition; the answer is the probability that it's true ("noul" in SystemOne).
	DecisionPredicate DecisionType = "predicate"
	// DecisionChoice picks one of Choices; the answer has the choice, a confidence and every choice's probability.
	DecisionChoice DecisionType = "choice"
	// DecisionScore places the input on ordered Levels (lowest first); the answer has the probability-weighted level
	// index, a confidence and every level's probability.
	DecisionScore DecisionType = "score"
)

// MaxDecisionQuestions and MaxDecisionOptions bound a request.
const (
	MaxDecisionQuestions = 64
	MaxDecisionOptions   = 255
)

// DecisionOption is one option of a choice question.
type DecisionOption struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// DecisionLevel is one level of a score question.
type DecisionLevel struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// DecisionQuestion is a question about the input. Name identifies its answer.
type DecisionQuestion struct {
	Type         DecisionType     `json:"type"`
	Name         string           `json:"name"`
	Instructions string           `json:"instructions"`
	Choices      []DecisionOption `json:"choices,omitempty"`
	Levels       []DecisionLevel  `json:"levels,omitempty"`
}

// DecideRequest asks a decision model its questions about Input.
type DecideRequest struct {
	ConnectionID string             `json:"connection_id"`
	Model        string             `json:"model,omitempty"`
	Input        string             `json:"input"`
	Questions    []DecisionQuestion `json:"questions"`
	Operation    string             `json:"operation,omitempty"`
}

// DecisionProbability is the probability of one choice (Value) or one level (Label, with its index in Value).
type DecisionProbability struct {
	Value       string  `json:"value,omitempty"`
	Label       string  `json:"label,omitempty"`
	Probability float64 `json:"probability"`
}

// DecisionAnswer answers the question called Name.
type DecisionAnswer struct {
	Name string       `json:"name"`
	Type DecisionType `json:"type"`
	// Probability is a predicate's probability of being true.
	Probability *float64 `json:"probability,omitempty"`
	// Choice is the picked value of a choice question.
	Choice string `json:"choice,omitempty"`
	// Score is a score question's probability-weighted level index (0 = the first level).
	Score         *float64              `json:"score,omitempty"`
	Confidence    *float64              `json:"confidence,omitempty"`
	Probabilities []DecisionProbability `json:"probabilities,omitempty"`
}

// DecideResponse has one answer per question, in the questions' order.
type DecideResponse struct {
	Model   string           `json:"model"`
	Answers []DecisionAnswer `json:"answers"`
	Usage   Usage            `json:"usage"`
}

// Answer gives the answer to the question called name.
func (r *DecideResponse) Answer(name string) (DecisionAnswer, bool) {
	for _, a := range r.Answers {
		if a.Name == name {
			return a, true
		}
	}
	return DecisionAnswer{}, false
}

// Validate checks a request's questions: named once each, of a known type, with their options.
func (r *DecideRequest) Validate() error {
	if len(r.Questions) == 0 {
		return errors.New("a decision needs at least one question")
	}
	if len(r.Questions) > MaxDecisionQuestions {
		return fmt.Errorf("a decision takes at most %d questions", MaxDecisionQuestions)
	}
	seen := make(map[string]bool, len(r.Questions))
	for _, q := range r.Questions {
		if q.Name == "" {
			return errors.New("every question needs a name")
		}
		if seen[q.Name] {
			return fmt.Errorf("the question name %q is used twice", q.Name)
		}
		seen[q.Name] = true
		switch q.Type {
		case DecisionPredicate:
		case DecisionChoice:
			if len(q.Choices) < 2 || len(q.Choices) > MaxDecisionOptions {
				return fmt.Errorf("the choice question %q needs between 2 and %d choices", q.Name, MaxDecisionOptions)
			}
			values := make(map[string]bool, len(q.Choices))
			for _, c := range q.Choices {
				if c.Value == "" || values[c.Value] {
					return fmt.Errorf("the choices of %q need distinct, non-empty values", q.Name)
				}
				values[c.Value] = true
			}
		case DecisionScore:
			if len(q.Levels) < 2 || len(q.Levels) > MaxDecisionOptions {
				return fmt.Errorf("the score question %q needs between 2 and %d levels", q.Name, MaxDecisionOptions)
			}
			for _, l := range q.Levels {
				if l.Label == "" {
					return fmt.Errorf("the levels of %q need labels", q.Name)
				}
			}
		default:
			return fmt.Errorf("the question %q has an unknown type %q", q.Name, q.Type)
		}
	}
	return nil
}
