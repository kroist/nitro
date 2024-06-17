//go:build !wasm
// +build !wasm

package precompiles

type RustLib struct {
	// Testf   func(string) string
	// Libtest uintptr
}

func InitLib() RustLib {
	// libtest, err := purego.Dlopen("/usr/local/bin/libtest_rust_ffi.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	// if err != nil {
	// 	panic(err)
	// }
	// defer purego.Dlclose(libtest)

	// Register the function
	return RustLib{
		// Testf: testf,
		// Libtest: libtest,
	}
}

func (con *RustLib) GetStr() (string, error) {
	return "Hi", nil
	// var testf func(string) string
	// purego.RegisterLibFunc(&testf, con.Libtest, "test_func")
	// return testf("hi"), nil
	// return con.Testf("hi"), nil
}
