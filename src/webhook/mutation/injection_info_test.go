package mutation

import (
	"testing"
)

func TestInjectionInfo_enabled(t *testing.T) {
	features := func() map[FeatureType]bool {
		i := NewInjectionInfo()
		i.add(Feature{
			featureType: OneAgent,
			enabled:     true,
		})
		i.add(Feature{
			featureType: DataIngest,
			enabled:     true,
		})
		return i.features
	}()

	type args struct {
		wanted FeatureType
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "",
			args: args{
				wanted: OneAgent,
			},
			want: true,
		},
		{
			name: "",
			args: args{
				wanted: DataIngest,
			},
			want: true,
		},
		{
			name: "",
			args: args{
				wanted: 999,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &InjectionInfo{
				features: features,
			}
			if got := info.enabled(tt.args.wanted); got != tt.want {
				t.Errorf("enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInjectionInfo_injectedAnnotation(t *testing.T) {
	type fields struct {
		features map[FeatureType]bool
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "OA-explicitlyEnabled",
			fields: fields{
				features: func() map[FeatureType]bool {
					i := NewInjectionInfo()
					i.add(Feature{
						featureType: OneAgent,
						enabled:     true,
					})
					return i.features
				}(),
			},
			want: "oneagent",
		},
		{
			name: "OA and DI explicitlyEnabled",
			fields: fields{
				features: func() map[FeatureType]bool {
					i := NewInjectionInfo()
					i.add(Feature{
						featureType: OneAgent,
						enabled:     true,
					})
					i.add(Feature{
						featureType: DataIngest,
						enabled:     true,
					})
					return i.features
				}(),
			},
			want: "data-ingest,oneagent",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &InjectionInfo{
				features: tt.fields.features,
			}
			if got := info.injectedAnnotation(); got != tt.want {
				t.Errorf("injectedAnnotation() = %v, want %v", got, tt.want)
			}
		})
	}
}
