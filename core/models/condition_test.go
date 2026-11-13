package models_test

import (
	"encoding/json"
	"testing"
	"time"

	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

var (
	t1 = time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	t2 = t1.Add(time.Minute)
)

func TestConditionsSet_AddsNew(t *testing.T) {
	g.RegisterTestingT(t)

	var conds models.Conditions
	conds.Set(models.Condition{
		Type:   models.ConditionVMMRunning,
		Status: models.ConditionTrue,
		Reason: models.ReasonRunning,
	}, t1)

	g.Expect(conds).To(g.HaveLen(1))
	g.Expect(conds[0].LastTransition).To(g.Equal(t1))
}

func TestConditionsSet_SameStatusKeepsTransition(t *testing.T) {
	g.RegisterTestingT(t)

	var conds models.Conditions
	conds.Set(models.Condition{Type: models.ConditionVMMRunning, Status: models.ConditionTrue, Reason: "a"}, t1)
	conds.Set(models.Condition{Type: models.ConditionVMMRunning, Status: models.ConditionTrue, Reason: "b", Message: "m"}, t2)

	g.Expect(conds).To(g.HaveLen(1))
	g.Expect(conds[0].Reason).To(g.Equal("b"))
	g.Expect(conds[0].Message).To(g.Equal("m"))
	g.Expect(conds[0].LastTransition).To(g.Equal(t1))
}

func TestConditionsSet_StatusChangeUpdatesTransition(t *testing.T) {
	g.RegisterTestingT(t)

	var conds models.Conditions
	conds.Set(models.Condition{Type: models.ConditionGuestAgentReady, Status: models.ConditionUnknown}, t1)
	conds.Set(models.Condition{Type: models.ConditionGuestAgentReady, Status: models.ConditionFalse}, t2)

	g.Expect(conds).To(g.HaveLen(1))
	g.Expect(conds[0].Status).To(g.Equal(models.ConditionFalse))
	g.Expect(conds[0].LastTransition).To(g.Equal(t2))
}

func TestConditionsGet_Missing(t *testing.T) {
	g.RegisterTestingT(t)

	var conds models.Conditions
	conds.Set(models.Condition{Type: models.ConditionVMMRunning, Status: models.ConditionTrue}, t1)

	_, ok := conds.Get(models.ConditionGuestAgentReady)
	g.Expect(ok).To(g.BeFalse())

	got, ok := conds.Get(models.ConditionVMMRunning)
	g.Expect(ok).To(g.BeTrue())
	g.Expect(got.Status).To(g.Equal(models.ConditionTrue))
}

func TestMicroVMStatus_DecodeWithoutConditions(t *testing.T) {
	g.RegisterTestingT(t)

	var status models.MicroVMStatus
	g.Expect(json.Unmarshal([]byte(`{"state":"created"}`), &status)).To(g.Succeed())
	g.Expect(status.Conditions).To(g.BeEmpty())
}
