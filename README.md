# lzma

一个从零实现中的 Go 版 LZMA（`.lzma` 裸流）解压器。

## 当前能力

- 支持 `.lzma` 头解析（properties / dictionary size / uncompressed size）
- 支持 range decoder（概率模型与 direct bits）
- 支持 literal / match / rep / short-rep 主解码路径
- 支持两种结束方式：
  - 已知解压长度：按头部 `UncompressedSize` 截止
  - 未知解压长度：通过 end marker 截止

## 暂不支持

- `.xz` 容器（块、索引、校验等）
- LZMA2

## 快速使用

```go
package main

import (
    "os"

    "github.com/nopdan/lzma"
)

func main() {
    compressed, _ := os.ReadFile("data.lzma")
    plain, err := lzma.Decompress(compressed)
    if err != nil {
        panic(err)
    }
    _ = os.WriteFile("data.out", plain, 0o644)
}
```

## 核心入口

- `Decompress(src []byte) ([]byte, error)`：一次性内存解压
- `(*Decoder).DecodeLZMA(r io.Reader, w io.Writer) error`：流式解压入口

## 测试

```bash
go test ./...
```

项目已包含基于参考实现生成压缩流的回归测试，用于验证 known-size 与 unknown-size(end marker) 两种场景。
