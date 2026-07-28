package handlers

import "testing"

func TestNormalizePatientConditionFields(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		diagnosis      string
		cancerDiameter string
		wantDiagnosis  string
		wantDiameter   string
		wantErr        bool
	}{
		{name: "healthy clears condition fields", status: 0, diagnosis: "旧诊断", cancerDiameter: "2.3", wantDiagnosis: "", wantDiameter: ""},
		{name: "sick keeps completed fields", status: 1, diagnosis: "肺癌", cancerDiameter: "2.3", wantDiagnosis: "肺癌", wantDiameter: "2.3"},
		{name: "sick requires diagnosis", status: 1, cancerDiameter: "2.3", wantErr: true},
		{name: "sick requires diameter", status: 1, diagnosis: "肺癌", wantErr: true},
		{name: "rejects invalid status", status: 2, diagnosis: "肺癌", cancerDiameter: "2.3", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnosis, diameter, err := normalizePatientConditionFields(tt.status, tt.diagnosis, tt.cancerDiameter)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if diagnosis != tt.wantDiagnosis {
				t.Errorf("diagnosis = %q, want %q", diagnosis, tt.wantDiagnosis)
			}
			if diameter != tt.wantDiameter {
				t.Errorf("diameter = %q, want %q", diameter, tt.wantDiameter)
			}
		})
	}
}
