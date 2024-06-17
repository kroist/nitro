package precompiles

// ArbHi provides a friendly greeting to anyone who calls it.
type ArbHi struct {
	Address addr // 0x11a, for example
	RLib    RustLib
}

func (con *ArbHi) SayHi(c ctx, evm mech) (string, error) {
	return con.RLib.GetStr()
}
