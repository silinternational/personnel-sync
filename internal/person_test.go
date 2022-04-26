package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPerson_Matches(t *testing.T) {
	tests := []struct {
		name    string
		person  Person
		filters Filters
		want    bool
		wantErr bool
	}{
		{
			name:    "no filters",
			person:  Person{},
			filters: nil,
			want:    true,
		},
		{
			name:    "simple filter, matches",
			person:  Person{Attributes: map[string]string{"attr": "value"}},
			filters: Filters{Filter{Attribute: "attr", Expression: "val"}},
			want:    true,
		},
		{
			name:    "simple filter, doesn't match",
			person:  Person{Attributes: map[string]string{"attr": "val"}},
			filters: Filters{Filter{Attribute: "attr", Expression: "value"}},
			want:    false,
		},
		{
			name: "complex filter, matches",
			person: Person{Attributes: map[string]string{
				"active": "true",
				"email":  "someone@example.com",
			}},
			filters: Filters{
				Filter{Attribute: "active", Expression: "true"},
				Filter{Attribute: "email", Expression: `@example\.com`},
			},
			want: true,
		},
		{
			name: "complex filter, matches first but not second filter",
			person: Person{Attributes: map[string]string{
				"active": "true",
				"email":  "someone@example.org",
			}},
			filters: Filters{
				Filter{Attribute: "active", Expression: "true"},
				Filter{Attribute: "email", Expression: `@not-matching-domain\.com`},
			},
			want: false,
		},
		{
			name: "complex filter, matches second but not first filter",
			person: Person{Attributes: map[string]string{
				"active": "false",
				"email":  "someone@example.com",
			}},
			filters: Filters{
				Filter{Attribute: "active", Expression: "true"},
				Filter{Attribute: "email", Expression: `@example\.com`},
			},
			want: false,
		},
		{
			name:    "missing attribute, not required",
			person:  Person{Attributes: map[string]string{"attr": "val"}},
			filters: Filters{Filter{Attribute: "other_attr", Expression: "value", Required: false}},
			wantErr: false,
		},
		{
			name:    "missing attribute, required",
			person:  Person{Attributes: map[string]string{"attr": "val"}},
			filters: Filters{Filter{Attribute: "other_attr", Expression: "value", Required: true}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.filters.Validate()
			require.NoError(t, err, "test configuration might be faulty")

			got, err := tt.person.Matches(tt.filters)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
