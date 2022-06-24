//go:build sweep
// +build sweep

package grafana

import (
	"github.com/aiven/terraform-provider-aiven/internal/sweep"
)

func init() {
	sweep.AddServiceSweeper("grafana")
}