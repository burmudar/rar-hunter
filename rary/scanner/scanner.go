package scanner

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/burmudar/rar-hunter/event"
	"github.com/burmudar/rar-hunter/eventbus"
)

type DirScanner struct {
	base string
	ctx  context.Context
}

func Scan(ctx context.Context, dir string) {
	workers := runtime.NumCPU()

	dirCh := make(chan string, workers*2)

	go func() {
		dirCh <- dir
	}()

	for _, w := range workers {
		go func() {
			for d := range dirCh {
				if ctx.Err() != nil {
					return
				}

				entries, err := os.ReadDir(d)
				if err != nil {
					eventbus.Publish(event.DirScanError{
						Root: d,
						Err:  error,
					})
				}
				for _, f := range entries {
					info, err := f.Info()
					if err != nil {
						eventbus.Publish(event.DirScanError{
							Root: d,
							Err:  error,
						})
					}
					if info.IsDir() && info.Name() != d {
						eventbus.Publish(event.DirFound{
							Root:     d,
							Name:     info.Name(),
							Absolute: filepath.Join(d, info.Name()),
						})
					}
				}
			}
		}()
	}

}

func (s *scanner) walk(path string, d fs.DirEntry, err error) error {
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
		if s.base == path {
			return nil
		}
		if d.IsDir() {
			eventbus.Publish(event.DirFound{
				Root:     s.base,
				Name:     d.Name(),
				Absolute: filepath.Join(path, d.Name()),
			})
		}
	}

	return nil
}
