package helm

import (
	"bytes"
	"fmt"

	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helm", func() {
	Describe("fanout", func() {
		It("should fail if the reader fails", func() {
			r := &failReadWriter{}
			w1 := gbytes.NewBuffer()
			written, err := fanout(r, w1)
			Expect(err).To(MatchError("failed to read"))
			Expect(written).To(Equal(int64(0)))
		})

		It("should fail when one of the writers fails", func() {
			const data = "foo"
			r := bytes.NewBufferString(data)
			w1 := gbytes.NewBuffer()
			w2 := &failReadWriter{}
			written, err := fanout(r, w1, w2)
			Expect(err).To(MatchError("failed to write"))
			Expect(written).To(Equal(int64(0)))
		})

		It("should write the data from the reader to the writers", func() {
			const data = "foo"
			r := bytes.NewBufferString(data)
			w1 := gbytes.NewBuffer()
			w2 := gbytes.NewBuffer()
			written, err := fanout(r, w1, w2)
			Expect(err).ToNot(HaveOccurred())
			Expect(written).To(Equal(int64(len(data))))
			Expect(w1).To(gbytes.Say(data))
			Expect(w2).To(gbytes.Say(data))
		})
	})
})

type failReadWriter struct{}

func (rw *failReadWriter) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("failed to read")
}

func (rw *failReadWriter) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("failed to write")
}
