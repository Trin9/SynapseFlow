package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"fault-playground/internal/faults"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: trigger_scenario <scenario_name>")
		os.Exit(1)
	}

	scenarioName := os.Args[1]
	
	config := faults.ScenarioConfig{
		Name: scenarioName,
		Fault: faults.FaultType(scenarioName),
	}

	// In a real playground, we'd send this to ALL services
	services := []string{"svc-a", "svc-b", "svc-c", "svc-d"}
	for _, svc := range services {
		url := fmt.Sprintf("http://%s:8080/admin/scenario", svc)
		sendScenario(url, config)
	}

	fmt.Printf("Scenario %s triggered across all services\n", scenarioName)
}

func sendScenario(url string, config faults.ScenarioConfig) {
	data, _ := json.Marshal(config)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		fmt.Printf("Failed to send scenario to %s: %v\n", url, err)
		return
	}
	defer resp.Body.Close()
	fmt.Printf("Sent scenario to %s: %s\n", url, resp.Status)
}
