//go:build wasm
// +build wasm

package precompiles

import (
	"context"
	_ "embed"
	"strconv"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type RustLib struct {
}

//go:embed external/rust/add.wasm
var addWasm []byte

func InitLib() RustLib {
	return RustLib{}
}

func (con *RustLib) GetStr() (string, error) {
	ctx := context.Background()
	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, r)
	mod, _ := r.Instantiate(ctx, addWasm)
	res, _ := mod.ExportedFunction("add").Call(ctx, 1, 2)
	return "test" + strconv.FormatUint(res[0], 10), nil
}
