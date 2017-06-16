package bytefmt_test

import (
	. "code.cloudfoundry.org/bytefmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("bytefmt", func() {

	Context("ByteSize", func() {
		It("Prints in the largest possible unit", func() {
			Expect(ByteSize(10 * TERABYTE)).To(Equal("10T"))
			Expect(ByteSize(uint64(10.5 * TERABYTE))).To(Equal("10.5T"))

			Expect(ByteSize(10 * GIGABYTE)).To(Equal("10G"))
			Expect(ByteSize(uint64(10.5 * GIGABYTE))).To(Equal("10.5G"))

			Expect(ByteSize(100 * MEGABYTE)).To(Equal("100M"))
			Expect(ByteSize(uint64(100.5 * MEGABYTE))).To(Equal("100.5M"))

			Expect(ByteSize(100 * KILOBYTE)).To(Equal("100K"))
			Expect(ByteSize(uint64(100.5 * KILOBYTE))).To(Equal("100.5K"))

			Expect(ByteSize(1)).To(Equal("1B"))
		})

		It("prints '0' for zero bytes", func() {
			Expect(ByteSize(0)).To(Equal("0"))
		})
	})

	Context("ToMegabytes", func() {
		It("parses byte amounts with short units (e.g. M, G)", func() {
			var (
				megabytes uint64
				err       error
			)

			megabytes, err = ToMegabytes("5B")
			Expect(megabytes).To(Equal(uint64(0)))
			Expect(err).NotTo(HaveOccurred())

			megabytes, err = ToMegabytes("5K")
			Expect(megabytes).To(Equal(uint64(0)))
			Expect(err).NotTo(HaveOccurred())

			megabytes, err = ToMegabytes("5M")
			Expect(megabytes).To(Equal(uint64(5)))
			Expect(err).NotTo(HaveOccurred())

			megabytes, err = ToMegabytes("5m")
			Expect(megabytes).To(Equal(uint64(5)))
			Expect(err).NotTo(HaveOccurred())

			megabytes, err = ToMegabytes("2G")
			Expect(megabytes).To(Equal(uint64(2 * 1024)))
			Expect(err).NotTo(HaveOccurred())

			megabytes, err = ToMegabytes("3T")
			Expect(megabytes).To(Equal(uint64(3 * 1024 * 1024)))
			Expect(err).NotTo(HaveOccurred())
		})

		It("parses byte amounts with long units (e.g MB, GB)", func() {
			var (
				megabytes uint64
				err       error
			)

			megabytes, err = ToMegabytes("5MB")
			Expect(megabytes).To(Equal(uint64(5)))
			Expect(err).NotTo(HaveOccurred())

			megabytes, err = ToMegabytes("5mb")
			Expect(megabytes).To(Equal(uint64(5)))
			Expect(err).NotTo(HaveOccurred())

			megabytes, err = ToMegabytes("2GB")
			Expect(megabytes).To(Equal(uint64(2 * 1024)))
			Expect(err).NotTo(HaveOccurred())

			megabytes, err = ToMegabytes("3TB")
			Expect(megabytes).To(Equal(uint64(3 * 1024 * 1024)))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error when the unit is missing", func() {
			_, err := ToMegabytes("5")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unit of measurement"))
		})

		It("returns an error when the unit is unrecognized", func() {
			_, err := ToMegabytes("5MBB")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unit of measurement"))

			_, err = ToMegabytes("5BB")
			Expect(err).To(HaveOccurred())
		})

		It("allows whitespace before and after the value", func() {
			megabytes, err := ToMegabytes("\t\n\r 5MB ")
			Expect(megabytes).To(Equal(uint64(5)))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error for negative values", func() {
			_, err := ToMegabytes("-5MB")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unit of measurement"))
		})

		It("returns an error for zero values", func() {
			_, err := ToMegabytes("0TB")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unit of measurement"))
		})
	})

	Context("ToBytes", func() {
		It("parses byte amounts with short units (e.g. M, G)", func() {
			var (
				bytes uint64
				err   error
			)

			bytes, err = ToBytes("5B")
			Expect(bytes).To(Equal(uint64(5)))
			Expect(err).NotTo(HaveOccurred())

			bytes, err = ToBytes("5K")
			Expect(bytes).To(Equal(uint64(5 * KILOBYTE)))
			Expect(err).NotTo(HaveOccurred())

			bytes, err = ToBytes("5M")
			Expect(bytes).To(Equal(uint64(5 * MEGABYTE)))
			Expect(err).NotTo(HaveOccurred())

			bytes, err = ToBytes("5m")
			Expect(bytes).To(Equal(uint64(5 * MEGABYTE)))
			Expect(err).NotTo(HaveOccurred())

			bytes, err = ToBytes("2G")
			Expect(bytes).To(Equal(uint64(2 * GIGABYTE)))
			Expect(err).NotTo(HaveOccurred())

			bytes, err = ToBytes("3T")
			Expect(bytes).To(Equal(uint64(3 * TERABYTE)))
			Expect(err).NotTo(HaveOccurred())
		})

		It("parses byte amounts that are float (e.g. 5.3KB)", func() {
			var (
				bytes uint64
				err   error
			)

			bytes, err = ToBytes("13.5KB")
			Expect(bytes).To(Equal(uint64(13824)))
			Expect(err).NotTo(HaveOccurred())

			bytes, err = ToBytes("4.5KB")
			Expect(bytes).To(Equal(uint64(4608)))
			Expect(err).NotTo(HaveOccurred())
		})

		It("parses byte amounts with long units (e.g MB, GB)", func() {
			var (
				bytes uint64
				err   error
			)

			bytes, err = ToBytes("5MB")
			Expect(bytes).To(Equal(uint64(5 * MEGABYTE)))
			Expect(err).NotTo(HaveOccurred())

			bytes, err = ToBytes("5mb")
			Expect(bytes).To(Equal(uint64(5 * MEGABYTE)))
			Expect(err).NotTo(HaveOccurred())

			bytes, err = ToBytes("2GB")
			Expect(bytes).To(Equal(uint64(2 * GIGABYTE)))
			Expect(err).NotTo(HaveOccurred())

			bytes, err = ToBytes("3TB")
			Expect(bytes).To(Equal(uint64(3 * TERABYTE)))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error when the unit is missing", func() {
			_, err := ToBytes("5")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unit of measurement"))
		})

		It("returns an error when the unit is unrecognized", func() {
			_, err := ToBytes("5MBB")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unit of measurement"))

			_, err = ToBytes("5BB")
			Expect(err).To(HaveOccurred())
		})

		It("allows whitespace before and after the value", func() {
			bytes, err := ToBytes("\t\n\r 5MB ")
			Expect(bytes).To(Equal(uint64(5 * MEGABYTE)))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error for negative values", func() {
			_, err := ToBytes("-5MB")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unit of measurement"))
		})

		It("returns an error for zero values", func() {
			_, err := ToBytes("0TB")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unit of measurement"))
		})
	})
})
