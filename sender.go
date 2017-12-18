package seth

import (
	"fmt"
	"math/big"
	"time"
)

// Sender is a client that sends transactions
// from a particular address.
type Sender struct {
	*Client
	Addr *Address

	// GasRatio is the ratio of the gas estimate
	// to use as the gas offered for a transaction,
	// expressed as a rational number.
	// For instance, Num=5,Denom=4 would offer 5/4ths of
	// the gas estimate as the gas for a transaction.
	GasRatio struct {
		Num, Denom int
	}

	// GasPrice is the gas price offered for each transaction.
	GasPrice Int
}

// NewSender constructs a Sender with sane defaults.
func NewSender(c *Client, from *Address) *Sender {
	s := &Sender{Client: c, Addr: from}
	s.GasRatio.Num = 6
	s.GasRatio.Denom = 5
	(*big.Int)(&s.GasPrice).SetString("5000000000", 10) // 5 Gwei
	return s
}

func (s *Sender) pad(gas *Int) *Int {
	if gas == nil {
		return nil
	}
	ob := new(big.Int)
	gb := (*big.Int)(gas)
	var num, denom big.Int
	num.SetInt64(int64(s.GasRatio.Num))
	denom.SetInt64(int64(s.GasRatio.Denom))
	ob.Set(gb)
	ob.Mul(ob, &num)
	ob.Div(ob, &denom)
	return (*Int)(ob)
}

func (s *Sender) ConstCall(to *Address, method string, out interface{}, args ...EtherType) error {
	opts := CallOpts{To: to, From: s.Addr, GasPrice: &s.GasPrice}
	opts.EncodeCall(method, args...)
	return s.Client.ConstCall(&opts, out, true)
}

// Create creates a new contract with the given contract code.
// This call blocks until the transaction posts, and then returns
// the contract's address.
func (s *Sender) Create(code []byte) (Address, error) {
	opts := CallOpts{From: s.Addr, GasPrice: &s.GasPrice}
	opts.Data = Data(code)
	gas, err := s.EstimateGas(&opts)
	if err != nil {
		return Address{}, err
	}
	opts.Gas = s.pad(&gas)
	h, err := s.Call(&opts)
	if err != nil {
		return Address{}, err
	}
	err = s.Wait(&h)
	if err != nil {
		return Address{}, err
	}
	r, err := s.GetReceipt(&h)
	if err != nil {
		return Address{}, err
	}
	if r.Address == nil {
		return Address{}, fmt.Errorf("txhash %s: contract not created", &h)
	}
	return *r.Address, nil
}

// Send makes a contract call from the sender address.
// It automatically handles gas estimation and padding.
func (s *Sender) Send(to *Address, method string, args ...EtherType) (Hash, error) {
	opts := CallOpts{To: to, From: s.Addr, GasPrice: &s.GasPrice}
	opts.EncodeCall(method, args...)
	gas, err := s.EstimateGas(&opts)
	if err != nil {
		return Hash{}, err
	}
	opts.Gas = s.pad(&gas)
	return s.Call(&opts)
}

// Wait waits for a transaction hash to be mined into the canonical chain.
func (s *Sender) Wait(h *Hash) error {
	for {
		t, err := s.GetTransaction(h)
		if err != nil {
			return err
		}
		if t.TxIndex != nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
}

// Drain waits for the pending transaction pool to
// contain no transactions from this account.
func (s *Sender) Drain(prompt ...func(t *Transaction)) error {
	for {
		p, err := s.GetBlock(-1, true)
		if err != nil {
			return err
		}
		txs, err := p.ParseTransactions()
		if err != nil {
			return err
		}
		var t *Transaction
		for i := range txs {
			if txs[i].From == s.Addr {
				t = &txs[i]
				break
			}
		}
		if t == nil {
			return nil
		}
		for _, p := range prompt {
			p(t)
		}
		if err := s.Wait(&t.Hash); err != nil {
			return err
		}
	}
}