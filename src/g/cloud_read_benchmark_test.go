package g

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
)

func BenchmarkCloudResponseRead(b *testing.B) {
	for _, size := range []int{128, 64 << 10, 1 << 20, maxCloudConfigBytes} {
		input := bytes.Repeat([]byte("x"), size)
		for _, active := range []bool{false, true} {
			ctx := context.Background()
			if active {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				b.Cleanup(cancel)
			}
			for _, known := range []bool{false, true} {
				b.Run(fmt.Sprintf("%dBytes/active=%v/known=%v", size, active, known), func(b *testing.B) {
					length := int64(-1)
					if known {
						length = int64(size)
					}
					b.ReportAllocs()
					b.SetBytes(int64(size))
					for i := 0; i < b.N; i++ {
						response := &http.Response{Body: io.NopCloser(bytes.NewReader(input)), ContentLength: length}
						got, err := readCloudConfigHTTPResponseBodyContext(ctx, response)
						if err != nil || !bytes.Equal(got, input) {
							b.Fatalf("read length=%d error=%v", len(got), err)
						}
					}
				})
			}
		}
	}
}
