package tsdb

import (
	"fmt"
	"sort"
	"testing"

	commonv1 "github.com/grafana/phlare/pkg/gen/common/v1"
	phlaremodel "github.com/grafana/phlare/pkg/model"
	"github.com/grafana/phlare/pkg/phlaredb/tsdb/shard"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func Test_GetShards(t *testing.T) {
	for _, tt := range []struct {
		total    uint32
		shard    *shard.Annotation
		expected []uint32
	}{
		// equal factors
		{16, &shard.Annotation{Shard: 0, Of: 16}, []uint32{0}},
		{16, &shard.Annotation{Shard: 4, Of: 16}, []uint32{4}},
		{16, &shard.Annotation{Shard: 15, Of: 16}, []uint32{15}},

		// idx factor a larger multiple of schema factor
		{32, &shard.Annotation{Shard: 0, Of: 16}, []uint32{0, 16}},
		{32, &shard.Annotation{Shard: 4, Of: 16}, []uint32{4, 20}},
		{32, &shard.Annotation{Shard: 15, Of: 16}, []uint32{15, 31}},
		{64, &shard.Annotation{Shard: 15, Of: 16}, []uint32{15, 31, 47, 63}},
	} {
		tt := tt
		t.Run(tt.shard.String()+fmt.Sprintf("_total_%d", tt.total), func(t *testing.T) {
			ii := NewWithShards(tt.total)
			res := ii.getShards(tt.shard)
			resInt := []uint32{}
			for _, r := range res {
				resInt = append(resInt, r.shard)
			}
			require.Equal(t, tt.expected, resInt)
		})
	}
}

func Test_ValidateShards(t *testing.T) {
	ii := NewWithShards(32)
	require.NoError(t, ii.validateShard(&shard.Annotation{Shard: 1, Of: 16}))
}

func TestDeleteAddLoopkup(t *testing.T) {
	index := NewWithShards(DefaultIndexShards)
	lbs := []*commonv1.LabelPair{
		{Name: "__name__", Value: "foo"},
		{Name: "foo", Value: "foo"},
		{Name: "bar", Value: "bar"},
		{Name: "buzz", Value: "buzz"},
	}
	sort.Sort(phlaremodel.Labels(lbs))

	require.Equal(t, uint32(6), labelsSeriesIDHash(lbs)%32)
	// make sure we consistent
	require.Equal(t, uint32(6), labelsSeriesIDHash(lbs)%32)
	index.Add(lbs, model.Fingerprint((phlaremodel.Labels(lbs).Hash())))
	index.Delete(lbs, model.Fingerprint(phlaremodel.Labels(lbs).Hash()))
	ids, err := index.Lookup([]*labels.Matcher{
		labels.MustNewMatcher(labels.MatchEqual, "foo", "foo"),
	}, nil)
	require.NoError(t, err)
	require.Len(t, ids, 0)
}

func Test_hash_mapping(t *testing.T) {
	lbs := []*commonv1.LabelPair{
		{Name: "compose_project", Value: "loki-boltdb-storage-s3"},
		{Name: "compose_service", Value: "ingester-2"},
		{Name: "container_name", Value: "loki-boltdb-storage-s3_ingester-2_1"},
		{Name: "filename", Value: "/var/log/docker/790fef4c6a587c3b386fe85c07e03f3a1613f4929ca3abaa4880e14caadb5ad1/json.log"},
		{Name: "host", Value: "docker-desktop"},
		{Name: "source", Value: "stderr"},
	}

	for _, shardID := range []uint32{16, 32, 64, 128} {
		t.Run(fmt.Sprintf("%d", shardID), func(t *testing.T) {
			ii := NewWithShards(shardID)
			ii.Add(lbs, 1)

			res, err := ii.Lookup([]*labels.Matcher{{Type: labels.MatchEqual, Name: "compose_project", Value: "loki-boltdb-storage-s3"}}, &shard.Annotation{Shard: int(labelsSeriesIDHash(lbs) % 16), Of: 16})
			require.NoError(t, err)
			require.Len(t, res, 1)
			require.Equal(t, model.Fingerprint(1), res[0])
		})
	}
}

func Test_NoMatcherLookup(t *testing.T) {
	lbs := []*commonv1.LabelPair{
		{Name: "foo", Value: "bar"},
		{Name: "hi", Value: "hello"},
	}
	// with no shard param
	ii := NewWithShards(16)
	ii.Add(lbs, 1)
	ids, err := ii.Lookup(nil, nil)
	require.Nil(t, err)
	require.Equal(t, model.Fingerprint(1), ids[0])

	// with shard param
	ii = NewWithShards(16)
	ii.Add(lbs, 1)
	ids, err = ii.Lookup(nil, &shard.Annotation{Shard: int(labelsSeriesIDHash(lbs) % 16), Of: 16})
	require.Nil(t, err)
	require.Equal(t, model.Fingerprint(1), ids[0])
}

func Test_ConsistentMapping(t *testing.T) {
	a := NewWithShards(16)
	b := NewWithShards(32)

	for i := 0; i < 100; i++ {
		lbs := []*commonv1.LabelPair{
			{Name: "foo", Value: "bar"},
			{Name: "hi", Value: fmt.Sprint(i)},
		}
		a.Add(lbs, model.Fingerprint(i))
		b.Add(lbs, model.Fingerprint(i))
	}

	shardMax := 8
	for i := 0; i < shardMax; i++ {
		shard := &shard.Annotation{
			Shard: i,
			Of:    shardMax,
		}

		aIDs, err := a.Lookup([]*labels.Matcher{
			labels.MustNewMatcher(labels.MatchEqual, "foo", "bar"),
		}, shard)
		require.Nil(t, err)

		bIDs, err := b.Lookup([]*labels.Matcher{
			labels.MustNewMatcher(labels.MatchEqual, "foo", "bar"),
		}, shard)
		require.Nil(t, err)

		sorter := func(xs []model.Fingerprint) {
			sort.Slice(xs, func(i, j int) bool {
				return xs[i] < xs[j]
			})
		}
		sorter(aIDs)
		sorter(bIDs)

		require.Equal(t, aIDs, bIDs, "incorrect shard mapping for shard %v", shard)
	}
}
