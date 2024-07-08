package models_test

import (
	"testing"

	"github.com/liquidmetal-dev/flintlock/core/models"
	g "github.com/onsi/gomega"
)

func TestIPAddressCIDR_IsIPv4(t *testing.T) {
	g.RegisterTestingT(t)

	type testCase struct {
		Name   string
		Input  string
		Result bool
		Error  string
	}

	cases := []testCase{
		{Input: "1.2.3.4/16", Result: true, Error: ""},
		{Input: "0.0.0.0/0", Result: true, Error: ""},
		{Input: "1.2.3.4", Result: false, Error: "invalid CIDR address"},
		{Input: "2.2/16", Result: false, Error: "invalid CIDR address"},
		{Input: "ajshdkja/16", Result: false, Error: "invalid CIDR address"},
		{Input: "255.255.255.255/16", Result: true, Error: ""},
		{Input: "999.999.999.999/16", Result: false, Error: "invalid CIDR address"},
		{Input: "2345:0425:2CA1:0000:0000:0567:5673:23b5/16", Result: false, Error: ""},
		{Input: "0:0:0:0:0:ffff:0101:0101/24", Result: false, Error: ""},
	}

	for _, test := range cases {
		address := models.IPAddressCIDR(test.Input)
		result, err := address.IsIPv4()

		g.Expect(result).To(
			g.Equal(test.Result),
			"Test on %s failed with '%s' error.",
			test.Input,
			err,
		)

		if test.Error == "" {
			g.Expect(err).NotTo(g.HaveOccurred())
		} else {
			g.Expect(err.Error()).To(g.ContainSubstring(test.Error))
		}
	}
}

func TestIPAddressCIDR_IP(t *testing.T) {
	g.RegisterTestingT(t)

	type testCase struct {
		Name   string
		Input  string
		Result string
		Error  string
	}

	cases := []testCase{
		{Input: "1.2.3.4/16", Result: "1.2.3.4", Error: ""},
		{Input: "0.0.0.0/0", Result: "0.0.0.0", Error: ""},
		{Input: "1.2.3.4", Result: "", Error: "invalid CIDR address"},
		{Input: "2.2/16", Result: "", Error: "invalid CIDR address"},
		{Input: "ajshdkja/16", Result: "", Error: "invalid CIDR address"},
		{Input: "255.255.255.255/16", Result: "255.255.255.255", Error: ""},
		{Input: "999.999.999.999/16", Result: "", Error: "invalid CIDR address"},
		{Input: "2001:db8::", Result: "", Error: "invalid CIDR address"},
		{Input: "2001:db8::/113", Result: "2001:db8::", Error: ""},
		{
			Input:  "0:0:0:0:0:ffff:0101:0101/24",
			Result: "0:0:0:0:0:ffff:0101:0101",
			Error:  "",
		},
		{
			Input:  "2345:0425:2CA1:0000:0000:0567:5673:23b5/32",
			Result: "2345:0425:2CA1:0000:0000:0567:5673:23b5",
			Error:  "",
		},
	}

	for _, test := range cases {
		address := models.IPAddressCIDR(test.Input)
		result, err := address.IP()

		g.Expect(result).To(
			g.Equal(test.Result),
			"Test on %s failed with '%s' error.",
			test.Input,
			err,
		)

		if test.Error == "" {
			g.Expect(err).NotTo(g.HaveOccurred())
		} else {
			g.Expect(err.Error()).To(g.ContainSubstring(test.Error))
		}
	}
}
