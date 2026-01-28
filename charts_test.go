package e2b

import (
	"testing"
)

func TestDeserializeLineChart(t *testing.T) {
	data := map[string]any{
		"type":          "line",
		"title":         "Test Line Chart",
		"x_label":       "X Axis",
		"y_label":       "Y Axis",
		"x_scale":       "linear",
		"y_scale":       "linear",
		"x_ticks":       []any{0.0, 1.0, 2.0},
		"x_tick_labels": []any{"0", "1", "2"},
		"y_ticks":       []any{0.0, 5.0, 10.0},
		"y_tick_labels": []any{"0", "5", "10"},
		"elements": []any{
			map[string]any{
				"label": "Series 1",
				"points": []any{
					[]any{0.0, 1.0},
					[]any{1.0, 4.0},
					[]any{2.0, 9.0},
				},
			},
		},
	}

	chart, err := DeserializeChart(data)
	if err != nil {
		t.Fatalf("DeserializeChart() error = %v", err)
	}

	if chart.ChartType() != ChartTypeLine {
		t.Errorf("ChartType() = %v, want %v", chart.ChartType(), ChartTypeLine)
	}

	if chart.ChartTitle() != "Test Line Chart" {
		t.Errorf("ChartTitle() = %v, want Test Line Chart", chart.ChartTitle())
	}

	lineChart, ok := chart.(*LineChart)
	if !ok {
		t.Fatal("chart is not a LineChart")
	}

	if lineChart.XLabel != "X Axis" {
		t.Errorf("XLabel = %v, want X Axis", lineChart.XLabel)
	}

	if len(lineChart.Elements) != 1 {
		t.Fatalf("Elements length = %d, want 1", len(lineChart.Elements))
	}

	if lineChart.Elements[0].Label != "Series 1" {
		t.Errorf("Elements[0].Label = %v, want Series 1", lineChart.Elements[0].Label)
	}

	if len(lineChart.Elements[0].Points) != 3 {
		t.Errorf("Elements[0].Points length = %d, want 3", len(lineChart.Elements[0].Points))
	}
}

func TestDeserializeBarChart(t *testing.T) {
	data := map[string]any{
		"type":    "bar",
		"title":   "Test Bar Chart",
		"x_label": "Category",
		"y_label": "Value",
		"elements": []any{
			map[string]any{
				"label": "A",
				"group": "Group 1",
				"value": 10.0,
			},
			map[string]any{
				"label": "B",
				"group": "Group 1",
				"value": 20.0,
			},
		},
	}

	chart, err := DeserializeChart(data)
	if err != nil {
		t.Fatalf("DeserializeChart() error = %v", err)
	}

	if chart.ChartType() != ChartTypeBar {
		t.Errorf("ChartType() = %v, want %v", chart.ChartType(), ChartTypeBar)
	}

	barChart, ok := chart.(*BarChart)
	if !ok {
		t.Fatal("chart is not a BarChart")
	}

	if len(barChart.Elements) != 2 {
		t.Fatalf("Elements length = %d, want 2", len(barChart.Elements))
	}

	if barChart.Elements[0].Label != "A" {
		t.Errorf("Elements[0].Label = %v, want A", barChart.Elements[0].Label)
	}

	if barChart.Elements[0].Group != "Group 1" {
		t.Errorf("Elements[0].Group = %v, want Group 1", barChart.Elements[0].Group)
	}
}

func TestDeserializePieChart(t *testing.T) {
	data := map[string]any{
		"type":  "pie",
		"title": "Test Pie Chart",
		"elements": []any{
			map[string]any{
				"label":  "Slice 1",
				"angle":  90.0,
				"radius": 1.0,
			},
			map[string]any{
				"label":  "Slice 2",
				"angle":  270.0,
				"radius": 1.0,
			},
		},
	}

	chart, err := DeserializeChart(data)
	if err != nil {
		t.Fatalf("DeserializeChart() error = %v", err)
	}

	if chart.ChartType() != ChartTypePie {
		t.Errorf("ChartType() = %v, want %v", chart.ChartType(), ChartTypePie)
	}

	pieChart, ok := chart.(*PieChart)
	if !ok {
		t.Fatal("chart is not a PieChart")
	}

	if len(pieChart.Elements) != 2 {
		t.Fatalf("Elements length = %d, want 2", len(pieChart.Elements))
	}

	if pieChart.Elements[0].Label != "Slice 1" {
		t.Errorf("Elements[0].Label = %v, want Slice 1", pieChart.Elements[0].Label)
	}

	if pieChart.Elements[0].Angle != 90.0 {
		t.Errorf("Elements[0].Angle = %v, want 90.0", pieChart.Elements[0].Angle)
	}
}

func TestDeserializeScatterChart(t *testing.T) {
	data := map[string]any{
		"type":          "scatter",
		"title":         "Test Scatter Chart",
		"x_label":       "X",
		"y_label":       "Y",
		"x_scale":       "linear",
		"y_scale":       "log",
		"x_ticks":       []any{},
		"x_tick_labels": []any{},
		"y_ticks":       []any{},
		"y_tick_labels": []any{},
		"elements": []any{
			map[string]any{
				"label": "Data Points",
				"points": []any{
					[]any{1.0, 2.0},
					[]any{3.0, 4.0},
				},
			},
		},
	}

	chart, err := DeserializeChart(data)
	if err != nil {
		t.Fatalf("DeserializeChart() error = %v", err)
	}

	if chart.ChartType() != ChartTypeScatter {
		t.Errorf("ChartType() = %v, want %v", chart.ChartType(), ChartTypeScatter)
	}

	scatterChart, ok := chart.(*ScatterChart)
	if !ok {
		t.Fatal("chart is not a ScatterChart")
	}

	if len(scatterChart.Elements) != 1 {
		t.Fatalf("Elements length = %d, want 1", len(scatterChart.Elements))
	}
}

func TestDeserializeBoxAndWhiskerChart(t *testing.T) {
	data := map[string]any{
		"type":    "box_and_whisker",
		"title":   "Test Box Chart",
		"x_label": "Category",
		"y_label": "Value",
		"elements": []any{
			map[string]any{
				"label":          "Box 1",
				"min":            1.0,
				"first_quartile": 2.0,
				"median":         3.0,
				"third_quartile": 4.0,
				"max":            5.0,
				"outliers":       []any{0.5, 5.5},
			},
		},
	}

	chart, err := DeserializeChart(data)
	if err != nil {
		t.Fatalf("DeserializeChart() error = %v", err)
	}

	if chart.ChartType() != ChartTypeBoxAndWhisker {
		t.Errorf("ChartType() = %v, want %v", chart.ChartType(), ChartTypeBoxAndWhisker)
	}

	boxChart, ok := chart.(*BoxAndWhiskerChart)
	if !ok {
		t.Fatal("chart is not a BoxAndWhiskerChart")
	}

	if len(boxChart.Elements) != 1 {
		t.Fatalf("Elements length = %d, want 1", len(boxChart.Elements))
	}

	if boxChart.Elements[0].Median != 3.0 {
		t.Errorf("Elements[0].Median = %v, want 3.0", boxChart.Elements[0].Median)
	}

	if len(boxChart.Elements[0].Outliers) != 2 {
		t.Errorf("Elements[0].Outliers length = %d, want 2", len(boxChart.Elements[0].Outliers))
	}
}

func TestDeserializeSuperChart(t *testing.T) {
	data := map[string]any{
		"type":  "superchart",
		"title": "Test Super Chart",
		"elements": []any{
			map[string]any{
				"type":          "line",
				"title":         "Sub Chart 1",
				"x_label":       "X",
				"y_label":       "Y",
				"x_ticks":       []any{},
				"x_tick_labels": []any{},
				"y_ticks":       []any{},
				"y_tick_labels": []any{},
				"elements":      []any{},
			},
			map[string]any{
				"type":     "bar",
				"title":    "Sub Chart 2",
				"x_label":  "X",
				"y_label":  "Y",
				"elements": []any{},
			},
		},
	}

	chart, err := DeserializeChart(data)
	if err != nil {
		t.Fatalf("DeserializeChart() error = %v", err)
	}

	if chart.ChartType() != ChartTypeSuperChart {
		t.Errorf("ChartType() = %v, want %v", chart.ChartType(), ChartTypeSuperChart)
	}

	superChart, ok := chart.(*SuperChart)
	if !ok {
		t.Fatal("chart is not a SuperChart")
	}

	if len(superChart.Elements) != 2 {
		t.Fatalf("Elements length = %d, want 2", len(superChart.Elements))
	}

	if superChart.Elements[0].ChartType() != ChartTypeLine {
		t.Errorf("Elements[0].ChartType() = %v, want line", superChart.Elements[0].ChartType())
	}

	if superChart.Elements[1].ChartType() != ChartTypeBar {
		t.Errorf("Elements[1].ChartType() = %v, want bar", superChart.Elements[1].ChartType())
	}
}

func TestDeserializeUnknownChart(t *testing.T) {
	data := map[string]any{
		"type":     "unknown_type",
		"title":    "Unknown Chart",
		"elements": []any{},
	}

	chart, err := DeserializeChart(data)
	if err != nil {
		t.Fatalf("DeserializeChart() error = %v", err)
	}

	if chart.ChartType() != ChartTypeUnknown {
		t.Errorf("ChartType() = %v, want %v", chart.ChartType(), ChartTypeUnknown)
	}
}

func TestDeserializeNilChart(t *testing.T) {
	chart, err := DeserializeChart(nil)
	if err != nil {
		t.Fatalf("DeserializeChart(nil) error = %v", err)
	}

	if chart != nil {
		t.Error("DeserializeChart(nil) should return nil")
	}
}

func TestChartToMap(t *testing.T) {
	data := map[string]any{
		"type":    "bar",
		"title":   "Test Chart",
		"x_label": "X",
		"y_label": "Y",
		"elements": []any{
			map[string]any{
				"label": "A",
				"group": "G1",
				"value": 10.0,
			},
		},
	}

	chart, err := DeserializeChart(data)
	if err != nil {
		t.Fatalf("DeserializeChart() error = %v", err)
	}

	result := chart.ToMap()
	if result["title"] != "Test Chart" {
		t.Errorf("ToMap()[title] = %v, want Test Chart", result["title"])
	}
}

func TestChartTypes(t *testing.T) {
	types := []ChartType{
		ChartTypeLine,
		ChartTypeScatter,
		ChartTypeBar,
		ChartTypePie,
		ChartTypeBoxAndWhisker,
		ChartTypeSuperChart,
		ChartTypeUnknown,
	}

	expected := []string{
		"line",
		"scatter",
		"bar",
		"pie",
		"box_and_whisker",
		"superchart",
		"unknown",
	}

	for i, ct := range types {
		if string(ct) != expected[i] {
			t.Errorf("ChartType = %v, want %v", ct, expected[i])
		}
	}
}

func TestScaleTypes(t *testing.T) {
	types := []ScaleType{
		ScaleTypeLinear,
		ScaleTypeDatetime,
		ScaleTypeCategorical,
		ScaleTypeLog,
		ScaleTypeSymlog,
		ScaleTypeLogit,
		ScaleTypeFunction,
		ScaleTypeFunctionLog,
		ScaleTypeAsinh,
		ScaleTypeUnknown,
	}

	expected := []string{
		"linear",
		"datetime",
		"categorical",
		"log",
		"symlog",
		"logit",
		"function",
		"functionlog",
		"asinh",
		"unknown",
	}

	for i, st := range types {
		if string(st) != expected[i] {
			t.Errorf("ScaleType = %v, want %v", st, expected[i])
		}
	}
}
