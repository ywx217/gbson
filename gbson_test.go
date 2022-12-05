package gbson

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

var (
	testLoadOnce sync.Once
	testLoad     []byte
)

func getTestLoad() []byte {
	testLoadOnce.Do(func() {
		body := bson.D{}
		for i := 0; i < 50; i++ {
			body = append(body, bson.E{Key: fmt.Sprintf("value-%d", i), Value: i})
		}
		for i := 0; i < 50; i++ {
			body = append(body, bson.E{Key: fmt.Sprintf("list-%d", i), Value: bson.A{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}})
		}
		var err error
		testLoad, err = bson.Marshal(body)
		if err != nil {
			panic(err)
		}
	})
	return testLoad
}

func TestGet(t *testing.T) {
	var m bson.D
	require.NoError(t, bson.Unmarshal(getTestLoad(), &m))

	for _, key := range []string{"list-0", "list-49"} {
		r := Get(getTestLoad(), key)
		require.True(t, r.Exist())
		require.Equal(t, BSONTypeArray, r.Type)
		arr := make([]int, 0, r.Length())
		r.IterArray(func(r Result) bool {
			arr = append(arr, int(r.Int64()))
			return true
		})
		require.Equal(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, arr)
	}
	require.Equal(t, int64(48), Get(getTestLoad(), "value-48").Int64())
}

func BenchmarkGetAllFields(b *testing.B) {
	var d bson.D
	load := getTestLoad()
	b.Logf("load size: %d bytes", len(load))
	require.NoError(b, bson.Unmarshal(load, &d))
	b.Run("bson unmarshal", func(b *testing.B) {
		// Unmarshal into bson.D using mongo-driver/bson
		for i := 0; i < b.N; i++ {
			var d bson.D
			require.NoError(b, bson.Unmarshal(load, &d))
		}
	})
	b.Run("gbson get all", func(b *testing.B) {
		// Gets all first level fields using gbson.Get
		for i := 0; i < b.N; i++ {
			for _, elem := range d {
				Get(load, elem.Key)
			}
		}
	})
	b.Run("gbson get first", func(b *testing.B) {
		// Gets the first single key with gbson.Get
		for i := 0; i < b.N; i++ {
			Get(load, d[0].Key)
		}
	})
	b.Run("gbson get last", func(b *testing.B) {
		// Gets the last single key with gbson.Get
		for i := 0; i < b.N; i++ {
			Get(load, d[len(d)-1].Key)
		}
	})
	b.Run("gbson map", func(b *testing.B) {
		// Parse the document into a map[string]Result using gbson.Map
		for i := 0; i < b.N; i++ {
			kvs := Get(load).Map()
			require.Equal(b, len(d), len(kvs))
		}
	})
	for _, multi := range []int{0, 1, 2} {
		b.Run(fmt.Sprintf("gbson sized *%d map", multi), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				kvs := Get(load).SizedMap(len(d) * multi)
				require.Equal(b, len(d), len(kvs))
			}
		})
	}
}
