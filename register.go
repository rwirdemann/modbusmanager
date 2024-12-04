package modbusmanager

type Register struct {
	SlaveAddress uint8  // the slave address to which this register belongs
	Address      uint16 // the address of this register
	Datatype     string // SINT16T12 | F32T1234 | T64T1234
	RegisterType string // coil | discrete | input | holding
	Action       string // read | write
	RawData      any
}
