package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalcPeriodStart(t *testing.T) {
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	week := int64(604800)

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "henüz oluşturuldu, elapsed=0",
			now:  createdAt,
			want: createdAt,
		},
		{
			name: "1. periyot ortası",
			now:  createdAt.Add(3*24*time.Hour + 12*time.Hour),
			want: createdAt,
		},
		{
			name: "tam 1 periyot doldu",
			now:  createdAt.Add(time.Duration(week) * time.Second),
			want: createdAt.Add(time.Duration(week) * time.Second),
		},
		{
			name: "1.5 periyot geçti, floor=1",
			now:  createdAt.Add(time.Duration(float64(week)*1.5) * time.Second),
			want: createdAt.Add(time.Duration(week) * time.Second),
		},
		{
			name: "tam 2 periyot doldu",
			now:  createdAt.Add(time.Duration(week*2) * time.Second),
			want: createdAt.Add(time.Duration(week*2) * time.Second),
		},
		{
			name: "2.99 periyot, floor=2",
			now:  createdAt.Add(time.Duration(float64(week)*2.99) * time.Second),
			want: createdAt.Add(time.Duration(week*2) * time.Second),
		},
		{
			name: "periyot sınırının 1 saniye öncesi",
			now:  createdAt.Add(time.Duration(week)*time.Second - time.Second),
			want: createdAt,
		},
		{
			name: "periyot sınırının 1 saniye sonrası",
			now:  createdAt.Add(time.Duration(week)*time.Second + time.Second),
			want: createdAt.Add(time.Duration(week) * time.Second),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcPeriodStart(createdAt, week, tt.now)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCalcNextResetAt(t *testing.T) {
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	week := int64(604800)

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "ilk periyotta → 1 hafta sonra",
			now:  createdAt.Add(3 * 24 * time.Hour),
			want: createdAt.Add(time.Duration(week) * time.Second),
		},
		{
			name: "2. periyotta → 2 hafta sonra",
			now:  createdAt.Add(time.Duration(week)*time.Second + time.Hour),
			want: createdAt.Add(time.Duration(week*2) * time.Second),
		},
		{
			name: "nextResetAt daima periodStart + interval",
			now:  createdAt.Add(time.Duration(float64(week)*5.7) * time.Second),
			want: calcPeriodStart(createdAt, week, createdAt.Add(time.Duration(float64(week)*5.7)*time.Second)).Add(time.Duration(week) * time.Second),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcNextResetAt(createdAt, week, tt.now)
			assert.Equal(t, tt.want, got)
		})
	}
}
