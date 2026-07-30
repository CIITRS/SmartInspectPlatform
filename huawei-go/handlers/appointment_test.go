package handlers

import "testing"

func TestSampleLocationText(t *testing.T) {
	tests := []struct {
		name, sampleStatus, expressStatus, direction, want string
	}{
		{"inbound transit", "collected", "transit", expressDirectionInbound, "患者样本寄往实验室途中"},
		{"outbound transit", "completed", "transit", expressDirectionOutbound, "报告/物料寄往患者途中"},
		{"received", "received", "delivered", expressDirectionInbound, "实验室已签收"},
		{"testing", "testing", "", "", "实验室检测中"},
		{"patient", "created", "", "", "患者处，待寄回"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sampleLocationText(tt.sampleStatus, tt.expressStatus, tt.direction); got != tt.want {
				t.Fatalf("sampleLocationText() = %q, want %q", got, tt.want)
			}
		})
	}
}
