package ebml

import (
	"fmt"
	"io"
	"log"
)

type limitedReadSeeker struct {
	*io.LimitedReader
}

func newLimitedReadSeeker(rs io.ReadSeeker, limit int64) *limitedReadSeeker {
	return &limitedReadSeeker{&io.LimitedReader{rs, limit}}
}

func (lrs *limitedReadSeeker) String() string {
	return fmt.Sprintf("%+v", lrs.LimitedReader)
}

func (lrs *limitedReadSeeker) Seek(offset int64, whence int) (ret int64, err error) {
	//	log.Println("seek0", lrs, offset, whence)
	s := lrs.LimitedReader.R.(io.Seeker)
	prevN := lrs.LimitedReader.N
	var curr int64
	curr, err = s.Seek(0, 1)
	if err != nil {
		log.Panic(err)
	}
	ret, err = s.Seek(offset, whence)
	if err != nil {
		log.Panic(err)
	}
	lrs.LimitedReader.N += curr - ret
	if offset == 0 && whence == 1 {
		if lrs.LimitedReader.N != prevN {
			log.Panic(lrs.LimitedReader.N, prevN, curr, ret)
		}
	}
	//	log.Println("seek1", lrs, offset, whence)
	return
}
