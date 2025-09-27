//go:build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	// 设置输出目录
	outputDir := "bin"

	// 创建输出目录
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// 要构建的模块
	modules := []struct {
		name    string
		path    string
		output  string
	}{
		{"control", "./cmd/control", "control"},
		{"simulator", "./cmd/simulator", "simulator"},
	}

	// 构建每个模块
	for _, mod := range modules {
		outputPath := filepath.Join(outputDir, mod.output)

		fmt.Printf("Building %s -> %s\n", mod.name, outputPath)

		cmd := exec.Command("go", "build", "-o", outputPath, mod.path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("Error building %s: %v\n", mod.name, err)
			os.Exit(1)
		}

		fmt.Printf("Successfully built %s\n", mod.name)
	}

	fmt.Println("All builds completed successfully!")
}