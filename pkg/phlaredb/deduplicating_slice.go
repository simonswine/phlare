package phlaredb

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	"github.com/segmentio/parquet-go"

	"github.com/grafana/phlare/pkg/phlaredb/block"
	schemav1 "github.com/grafana/phlare/pkg/phlaredb/schemas/v1"
	"github.com/grafana/phlare/pkg/util/build"
)

var int64SlicePool = &sync.Pool{
	New: func() interface{} {
		return make([]int64, 0)
	},
}

var defaultParquetConfig = &ParquetConfig{
	MaxBufferRowCount: 100_000,
	MaxRowGroupBytes:  128 * 1024 * 1024,
	MaxBlockBytes:     10 * 128 * 1024 * 1024,
}

type deduplicatingSlice[M Models, K comparable, H Helper[M, K], P schemav1.Persister[*M]] struct {
	lock   sync.RWMutex
	lookup map[K]int64

	persister P
	helper    H

	file   *os.File
	cfg    *ParquetConfig
	writer *parquet.Writer

	buffer      *parquet.GenericBuffer[*M]
	appendCh    chan *appendElems[M]
	rowsFlushed int
}

func (s *deduplicatingSlice[M, K, H, P]) Name() string {
	return s.persister.Name()
}

func (s *deduplicatingSlice[M, K, H, P]) Size() uint64 {
	return uint64(s.buffer.Size())
}

func (s *deduplicatingSlice[M, K, H, P]) Init(path string, cfg *ParquetConfig) error {
	s.cfg = cfg
	file, err := os.OpenFile(filepath.Join(path, s.persister.Name()+block.ParquetSuffix), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	s.file = file

	// TODO: Reuse parquet.Writer beyond life time of the head.
	s.writer = parquet.NewWriter(file, s.persister.Schema(),
		parquet.ColumnPageBuffers(parquet.NewFileBufferPool(os.TempDir(), "phlaredb-parquet-buffers*")),
		parquet.CreatedBy("github.com/grafana/phlare/", build.Version, build.Revision),
	)
	s.lookup = make(map[K]int64)
	// TODO: Review the 32 buffer
	s.appendCh = make(chan *appendElems[M], 32)

	// initialize the buffer
	s.buffer = parquet.NewGenericBuffer[*M](
		s.persister.Schema(),
		parquet.SortingRowGroupConfig(s.persister.SortingColumns()),
		parquet.ColumnBufferCapacity(s.cfg.MaxBufferRowCount),
	)

	// start goroutine for ingest
	// TODO: Should be shutdown properly at exist
	go s.appendLoop()

	return nil
}

func (s *deduplicatingSlice[M, K, H, P]) Close() error {
	if err := s.writer.Close(); err != nil {
		return errors.Wrap(err, "closing parquet writer")
	}

	if err := s.file.Close(); err != nil {
		return errors.Wrap(err, "closing parquet file")
	}

	return nil
}

func (s *deduplicatingSlice[M, K, H, P]) maxRowsPerRowGroup() int {
	numRows := s.buffer.NumRows()
	// with empty slice we need to return early
	if numRows == 0 {
		return 1
	}

	var (
		// average size per row in memory
		bytesPerRow = s.Size() / uint64(numRows)

		// how many rows per RG with average size are fitting in the maxRowGroupBytes, ensure that we at least flush 1 row
		maxRows = s.cfg.MaxRowGroupBytes / bytesPerRow
	)

	if maxRows <= 0 {
		return 1
	}

	return int(maxRows)
}

func (s *deduplicatingSlice[M, K, H, P]) Flush() (numRows uint64, numRowGroups uint64, err error) {
	return 0, 0, err
	/*
		s.lock.Lock()
		defer s.lock.Unlock()

		var (
			maxRows = s.maxRowsPerRowGroup()

			rowGroupsFlushed int
			rowsFlushed      int

		for {
			// how many rows of the head still in need of flushing
			rowsToFlush := len(s.slice) - s.rowsFlushed

			if rowsToFlush == 0 {
				break
			}

			// cap max row size by bytes
			if rowsToFlush > maxRows {
				rowsToFlush = maxRows
			}
			// cap max row size by buffer
			if rowsToFlush > s.cfg.MaxBufferRowCount {
				rowsToFlush = s.cfg.MaxBufferRowCount

			}

			rows := make([]parquet.Row, rowsToFlush)
			var slicePos int
			for pos := range rows {
				slicePos = pos + s.rowsFlushed
				rows[pos] = s.persister.Deconstruct(rows[pos], uint64(slicePos), s.slice[slicePos])
			}

			s.buffer.Reset()
			if _, err := s.buffer.WriteRows(rows); err != nil {
				return 0, 0, err
			}

			sort.Sort(s.buffer)

			if _, err := s.writer.WriteRowGroup(s.buffer); err != nil {
				return 0, 0, err
			}

			s.rowsFlushed += rowsToFlush
			rowsFlushed += rowsToFlush
			rowGroupsFlushed++
		}

		return uint64(rowsFlushed), uint64(rowGroupsFlushed), nil
	*/
}

// TODO: Remove me, bad idea
func (s *deduplicatingSlice[M, K, H, P]) Slice() []*M {
	var (
		mPtr   = make([]*M, s.buffer.Len())
		mReal  = make([]M, s.buffer.Len())
		rows   = make([]parquet.Row, s.buffer.Len())
		reader = s.buffer.Rows()
	)
	defer reader.Close()

	if _, err := reader.ReadRows(rows); err != nil {
		panic(err)
	}

	for pos := range rows {
		reader.Schema().Reconstruct(&mReal[pos], rows[pos])
		mPtr[pos] = &mReal[pos]
	}

	return mPtr
}

func (s *deduplicatingSlice[M, K, H, P]) GetRowNum(rowNum uint64) *M {
	var (
		m      M
		row    = make([]parquet.Row, 1)
		reader = s.buffer.Rows()
	)
	defer reader.Close()

	if err := reader.SeekToRow(int64(rowNum)); err != nil {
		panic(err)
	}

	if _, err := reader.ReadRows(row); err != nil {
		panic(err)
	}

	if err := reader.Schema().Reconstruct(&m, row[0]); err != nil {
		panic(err)
	}

	return &m
}

func (s *deduplicatingSlice[M, K, H, P]) isDeduplicating() bool {
	var k K
	return !isNoKey(k)
}

type appendElems[M Models] struct {
	elems        []*M
	rewritingMap map[int64]int64
	originalPos  []int64
	done         chan struct{}
	err          error
}

func (s *deduplicatingSlice[M, K, H, P]) filterAlreadyExistingElems(elems *appendElems[M]) error {

	for pos := range elems.elems {
		k := s.helper.key(elems.elems[pos])
		if posSlice, exists := s.lookup[k]; exists {
			elems.rewritingMap[int64(s.helper.setID(uint64(pos), uint64(posSlice), elems.elems[pos]))] = posSlice
		} else {
			elems.elems[len(elems.originalPos)] = elems.elems[pos]
			elems.originalPos = append(elems.originalPos, int64(pos))
		}
	}

	// reset slice to only contain missing elements
	elems.elems = elems.elems[:len(elems.originalPos)]

	return nil
}

// append loop is used to serialize the append and avoid locking
func (s *deduplicatingSlice[M, K, H, P]) appendLoop() {
	// TODO: Sort out graceful shutdown.

	for elems := range s.appendCh {
		if s.isDeduplicating() {
			s.filterAlreadyExistingElems(elems)
		}

		// all elements already exist
		if len(elems.elems) == 0 {
			close(elems.done)
			continue
		}

		numRows := s.buffer.NumRows()

		// append rows to buffer
		_, err := s.buffer.Write(elems.elems)
		if err != nil {
			elems.err = err
			close(elems.done)
			continue
		}

		// update hashmap and add rewrite information
		for pos := range elems.elems {
			k := s.helper.key(elems.elems[pos])
			var (
				previousPos = uint64(pos)
				newPos      = numRows + int64(pos)
			)
			s.lookup[k] = newPos
			if s.isDeduplicating() {
				previousPos = uint64(elems.originalPos[pos])
			}
			elems.rewritingMap[int64(s.helper.setID(previousPos, uint64(newPos), elems.elems[pos]))] = newPos
		}

		// close done channel
		close(elems.done)
	}

}

func (s *deduplicatingSlice[M, K, H, P]) ingest(ctx context.Context, elems []*M, rewriter *rewriter) error {
	var appendElems = &appendElems[M]{
		rewritingMap: make(map[int64]int64),
		done:         make(chan struct{}),
		originalPos:  make([]int64, 0, len(elems)),
		elems:        elems,
	}

	// rewrite elements
	for pos := range appendElems.elems {
		if err := s.helper.rewrite(rewriter, appendElems.elems[pos]); err != nil {
			return err
		}
	}

	// append to write channel
	s.appendCh <- appendElems

	<-appendElems.done

	if err := appendElems.err; err != nil {
		return err
	}

	// add rewrite information to struct
	s.helper.addToRewriter(rewriter, appendElems.rewritingMap)

	return nil
}

func (s *deduplicatingSlice[M, K, H, P]) NumRows() uint64 {
	return uint64(s.buffer.NumRows())
}

func (s *deduplicatingSlice[M, K, H, P]) getIndex(key K) (int64, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	v, ok := s.lookup[key]
	return v, ok
}
