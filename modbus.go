package modbusmanager

import (
	"fmt"
	"github.com/simonvetter/modbus"
	"log"
	"log/slog"
	"time"
)

var client *modbus.ModbusClient

func Init(config Config) {
	slog.Info("Initializing Modbus")
	serial := config.Serial[0]
	var err error
	client, err = modbus.NewClient(&modbus.ClientConfiguration{
		URL:      serial.Url,
		Speed:    uint(serial.Speed),
		DataBits: uint(serial.DataBits),
		Parity:   uint(serial.Parity),
		StopBits: uint(serial.StopBits),
		Timeout:  time.Duration(serial.Timeout) * time.Millisecond,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err = client.Open(); err != nil {
		log.Fatal(err)
	}
}

func WriteRegister(r Register) error {
	if err := client.SetUnitId(r.SlaveAddress); err != nil {
		return err
	}

	switch r.Datatype {
	case "BOOL":
		v := r.RawData.(bool)
		if err := client.WriteCoil(r.Address, v); err != nil {
			return err
		}
	case "F32T1234":
		v := r.RawData.(float32)
		if err := client.WriteFloat32(r.Address, v); err != nil {
			return err
		}
	default:
		panic(fmt.Sprintf("WriteRegisters: Unknown data type [type=%s]", r.Datatype))
	}
	return nil
}

func ReadDiscrete(register []Register) ([]Register, error) {
	if len(register) == 0 {
		return []Register{}, nil
	}

	if err := client.SetUnitId(register[0].SlaveAddress); err != nil {
		return []Register{}, err
	}

	var rr []Register
	for _, r := range register {
		b, err := client.ReadDiscreteInput(r.Address)
		if err != nil {
			return []Register{}, err
		}
		r.RawData = b
		rr = append(rr, r)
	}
	return rr, nil
}

func ReadInput(input []Register, i int) ([]Register, error) {
	return nil, nil

}

func ReadHolding(holding []Register, i int) ([]Register, error) {
	return nil, nil
}
