package modbusmanager

import (
	"fmt"
	"github.com/simonvetter/modbus"
	"log/slog"
	"time"
)

type Adapter struct {
	client *modbus.ModbusClient
}

func NewAdapter(serial Serial) (Adapter, error) {
	client, err := modbus.NewClient(&modbus.ClientConfiguration{
		URL:      serial.Url,
		Speed:    uint(serial.Speed),
		DataBits: uint(serial.DataBits),
		Parity:   uint(serial.Parity),
		StopBits: uint(serial.StopBits),
		Timeout:  time.Duration(serial.Timeout) * time.Millisecond,
	})

	if err != nil {
		return Adapter{}, err
	}
	if err = client.Open(); err != nil {
		return Adapter{}, err
	}
	slog.Info("modbus configuration updated and reconnected to", "url", serial.Url)
	return Adapter{client: client}, nil
}

func (a Adapter) WriteRegister(r Register) error {
	if err := a.client.SetUnitId(r.SlaveAddress); err != nil {
		return err
	}

	switch r.Datatype {
	case "BOOL":
		v := r.RawData.(bool)
		if err := a.client.WriteCoil(r.Address, v); err != nil {
			return err
		}
	case "F32T1234":
		v := r.RawData.(float32)
		if err := a.client.WriteFloat32(r.Address, v); err != nil {
			return err
		}
	default:
		panic(fmt.Sprintf("WriteRegisters: Unknown data type [type=%s]", r.Datatype))
	}
	return nil
}

func (a Adapter) ReadDiscrete(register []Register) ([]Register, error) {
	if len(register) == 0 {
		return []Register{}, nil
	}

	if err := a.client.SetUnitId(register[0].SlaveAddress); err != nil {
		return []Register{}, err
	}

	var rr []Register
	for _, r := range register {
		b, err := a.client.ReadDiscreteInput(r.Address)
		if err != nil {
			return []Register{}, err
		}
		r.RawData = b
		rr = append(rr, r)
	}
	return rr, nil
}

func (a Adapter) ReadInput(input []Register, i int) ([]Register, error) {
	return nil, nil

}

func (a Adapter) ReadHolding(holding []Register, i int) ([]Register, error) {
	return nil, nil
}
