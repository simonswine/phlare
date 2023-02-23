package main

import (
	"fmt"

	"github.com/apache/arrow/go/arrow"
	"github.com/apache/arrow/go/arrow/array"
	"github.com/apache/arrow/go/arrow/memory"
)

func main() {
	pool := memory.NewGoAllocator()

	sample := arrow.StructOf([]arrow.Field{
		{Name: "StacktraceID", Type: arrow.PrimitiveTypes.Uint64},
		{Name: "Value", Type: arrow.PrimitiveTypes.Int64},
		// TODO: Add labels here
	}...)

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "Samples", Type: arrow.ListOf(sample)},
		},
		nil,
	)

	b := array.NewRecordBuilder(pool, schema)
	defer b.Release()

	lb := b.Field(0).(*array.ListBuilder)
	sb := lb.ValueBuilder().(*array.StructBuilder)
	lb.Append(true)
	sb.AppendValues([]bool{true, true, true})
	sb.FieldBuilder(0).(*array.Uint64Builder).AppendValues([]uint64{1, 2, 3}, nil)
	sb.FieldBuilder(1).(*array.Int64Builder).AppendValues([]int64{10, 20, 30}, nil)

	rec := b.NewRecord()
	defer rec.Release()

	for i, col := range rec.Columns() {
		fmt.Printf("column[%d] %q: %v\n", i, rec.ColumnName(i), col)
	}

}
