package mutation

import "testing"

func TestFeature_name(t *testing.T) {
	type fields struct {
		ftype FeatureType
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "",
			fields: fields{
				ftype: OneAgent,
			},
			want: "oneagent",
		},
		{
			name: "",
			fields: fields{
				ftype: DataIngest,
			},
			want: "data-ingest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := Feature{
				featureType: tt.fields.ftype,
			}
			if got := f.featureType.name(); got != tt.want {
				t.Errorf("name() = %v, want %v", got, tt.want)
			}
		})
	}
}
