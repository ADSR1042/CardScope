//go:build !linux

package collect

import (
	"fmt"
	"gpu-monitor/internal/model"
)

func nvInventory() ([]model.GPU, error) { return nil, fmt.Errorf("Linux required") }
func nvGPU(u string) (model.GPU, error) { return model.GPU{}, fmt.Errorf("Linux required") }
