// Copyright (C) 2026 Ioannis Torakis <john.torakis@gmail.com>
// SPDX-License-Identifier: Elastic-2.0
//
// Licensed under the Elastic License 2.0.
// You may obtain a copy of the license at:
// https://www.elastic.co/licensing/elastic-license
//
// Use, modification, and redistribution permitted under the terms of the license,
// except for providing this software as a commercial service or product.

package table

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/acarl005/stripansi"
	"github.com/olekukonko/tablewriter"
)

// TableOptions configures table rendering behavior
type TableOptions struct {
	Headers []string
	SortBy  int // Column index to sort by (0-based), -1 for no sorting
	GroupBy int // Column index to group by (0-based), -1 for no grouping
}

// Row represents a table row as a slice of strings
type Row []string

// NewTable creates a new configured table with the given options
func NewTable(options TableOptions) *tablewriter.Table {
	// Create table with the custom symbols and auto-wrap config
	table := tablewriter.NewTable(
		os.Stdout,
		tablewriter.WithConfig(tablewriter.Config{
			// TODO: Truncate long strings but not on some columns (e.g. not the gate)
			// import: "github.com/olekukonko/tablewriter/tw"
			// Row: tw.CellConfig{
			// 	Formatting:   tw.CellFormatting{AutoWrap: tw.WrapTruncate}, // Wrap long content
			// 	Alignment:    tw.CellAlignment{Global: tw.AlignLeft},     // Left-align rows
			// 	ColMaxWidths: tw.CellWidth{Global: 40},                   // Max width per column
			// },
		}),
	)

	// Set headers - convert []string to []any
	headers := make([]any, len(options.Headers))
	for i, h := range options.Headers {
		headers[i] = h
	}
	table.Header(headers...)

	return table
}

// RenderTable renders a table with the given options and rows, handling sorting and grouping
func RenderTable(options TableOptions, rows []Row) {
	if len(rows) == 0 {
		return
	}

	// Sort rows if requested
	if options.SortBy >= 0 && options.SortBy < len(options.Headers) {
		sort.Slice(rows, func(i, j int) bool {
			if options.SortBy >= len(rows[i]) || options.SortBy >= len(rows[j]) {
				return false
			}
			// Case-insensitive sort, stripping color codes
			a := stripansi.Strip(rows[i][options.SortBy])
			b := stripansi.Strip(rows[j][options.SortBy])
			return strings.ToLower(a) < strings.ToLower(b)
		})
	}

	// Group rows if requested
	if options.GroupBy >= 0 && options.GroupBy < len(options.Headers) {
		rows = groupRows(rows, options.GroupBy)
	}

	table := NewTable(options)

	// Convert Row type to []any for Bulk method
	data := make([][]any, len(rows))
	for i, row := range rows {
		data[i] = make([]any, len(row))
		for j, cell := range row {
			data[i][j] = cell
		}
	}

	if err := table.Bulk(data); err != nil {
		fmt.Printf("Warning: failed to set table data: %v\n", err)
	}
	if err := table.Render(); err != nil {
		fmt.Printf("Warning: failed to render table: %v\n", err)
	}
}

func groupRows(rows []Row, groupByColumn int) []Row {
	if len(rows) == 0 {
		return rows
	}

	grouped := make([]Row, 0, len(rows))
	var lastGroupValue string

	for _, row := range rows {
		if groupByColumn >= len(row) {
			grouped = append(grouped, row)
			continue
		}

		currentGroupValue := stripansi.Strip(row[groupByColumn])

		if currentGroupValue == lastGroupValue && len(grouped) > 0 {
			// Replace the group column with empty string for duplicate entries
			newRow := make(Row, len(row))
			copy(newRow, row)
			newRow[groupByColumn] = ""
			grouped = append(grouped, newRow)
		} else {
			grouped = append(grouped, row)
			lastGroupValue = currentGroupValue
		}
	}

	return grouped
}
