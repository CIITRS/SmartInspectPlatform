package handlers

import (
	"reflect"
	"strings"
	"testing"
)

func TestAppendPatientKeywordFilter(t *testing.T) {
	query, args := appendPatientKeywordFilter("SELECT * FROM detect_patient WHERE is_active = ?", []interface{}{1}, " 包桂芝 ")

	for _, column := range []string{"name LIKE ?", "phone LIKE ?", "patient_code LIKE ?", "id_document_no", "id_card"} {
		if !strings.Contains(query, column) {
			t.Fatalf("keyword query does not search %s: %s", column, query)
		}
	}
	wantArgs := []interface{}{1, "%包桂芝%", "%包桂芝%", "%包桂芝%", "%包桂芝%", "%包桂芝%"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args: got %#v, want %#v", args, wantArgs)
	}
}

func TestAppendPatientKeywordFilterIgnoresBlankKeyword(t *testing.T) {
	const originalQuery = "SELECT * FROM detect_patient WHERE is_active = ?"
	originalArgs := []interface{}{1}
	query, args := appendPatientKeywordFilter(originalQuery, originalArgs, "  ")

	if query != originalQuery || !reflect.DeepEqual(args, originalArgs) {
		t.Fatalf("blank keyword changed query or args: %q %#v", query, args)
	}
}

func TestSalesPatientAccessFilter(t *testing.T) {
	tests := []struct {
		name      string
		roles     []string
		code      string
		alias     string
		wantQuery string
		wantArgs  []interface{}
	}{
		{name: "sales sees own patients", roles: []string{"销售"}, code: "23005", alias: "p", wantQuery: "p.sales_person = ?", wantArgs: []interface{}{"23005"}},
		{name: "sales without employee code fails closed", roles: []string{"销售"}, wantQuery: "1 = 0"},
		{name: "administrator is unrestricted", roles: []string{"管理员", "销售"}, code: "admin"},
		{name: "non-sales employee is unrestricted here", roles: []string{"检验师"}, code: "23009"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args := salesPatientAccessFilter(tt.roles, tt.code, tt.alias)
			if query != tt.wantQuery || !reflect.DeepEqual(args, tt.wantArgs) {
				t.Fatalf("got %q %#v, want %q %#v", query, args, tt.wantQuery, tt.wantArgs)
			}
		})
	}
}
