package main

func modbusCRC(msg []byte) uint16 {
	err := uint16(0xffff)
	for _, b := range msg {
		lb := byte(err & 0xff)
		err = err&0xff00 + uint16(lb^b)
		for bit := 0; bit < 8; bit++ {
			lsb := err & 0x0001
			if lsb == 1 {
				err--
			}
			err /= 2
			if lsb == 1 {
				err ^= 0xA001
			}
		}
	}
	return err
}
