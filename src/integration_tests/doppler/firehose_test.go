package doppler_test

import (
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gorilla/websocket"
	"github.com/nu7hatch/gouuid"
	"net"
	"time"

	. "integration_tests/doppler/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Firehose test", func() {
	var inputConnection net.Conn
	var appID string

	BeforeEach(func() {

		guid, _ := uuid.NewV4()
		appID = guid.String()

		inputConnection, _ = net.Dial("udp", localIPAddress+":8765")
		time.Sleep(50 * time.Millisecond) // give time for connection to establish
	})

	AfterEach(func() {
		inputConnection.Close()
	})
	Context("a single firehose gets all types of logs", func() {
		var ws *websocket.Conn
		var receiveChan chan []byte
		BeforeEach(func() {
			receiveChan = make(chan []byte, 10)
			ws, _ = AddWSSink(receiveChan, "4567", "/firehose/hose-subcription-a")
		})
		AfterEach(func() {
			ws.Close()
		})
		It("receives log messages", func() {
			SendAppLog(appID, "message", inputConnection)

			receivedMessageBytes := []byte{}
			Eventually(receiveChan).Should(Receive(&receivedMessageBytes))

			receivedMessage := DecodeProtoBufLogMessage(receivedMessageBytes)
			Expect(*receivedMessage).To(BeAssignableToTypeOf(events.LogMessage{}))
		})

		It("receives container metrics", func() {
			containerMetric := factories.NewContainerMetric(appID, 0, 10, 2, 3)
			SendEvent(containerMetric, inputConnection)

			receivedMessageBytes := []byte{}
			Eventually(receiveChan).Should(Receive(&receivedMessageBytes))

			receivedMessage := UnmarshalMessage(receivedMessageBytes)
			Expect(*receivedMessage.ContainerMetric).To(BeAssignableToTypeOf(events.ContainerMetric{}))
		})
	})

	It("two separate firehose subscriptions receive the same message", func() {
		receiveChan1 := make(chan []byte, 10)
		receiveChan2 := make(chan []byte, 10)
		firehoseWs1, _ := AddWSSink(receiveChan1, "4567", "/firehose/hose-subscription-1")
		firehoseWs2, _ := AddWSSink(receiveChan2, "4567", "/firehose/hose-subscription-2")
		defer firehoseWs1.Close()
		defer firehoseWs2.Close()

		SendAppLog(appID, "message", inputConnection)

		receivedMessageBytes1 := []byte{}
		Eventually(receiveChan1).Should(Receive(&receivedMessageBytes1))

		receivedMessageBytes2 := []byte{}
		Eventually(receiveChan2).Should(Receive(&receivedMessageBytes2))

		receivedMessage1 := DecodeProtoBufLogMessage(receivedMessageBytes1)
		Expect(string(receivedMessage1.GetMessage())).To(Equal("message"))

		receivedMessage2 := DecodeProtoBufLogMessage(receivedMessageBytes2)
		Expect(string(receivedMessage2.GetMessage())).To(Equal("message"))
	})

	It("firehose subscriptions split message load", func() {
		receiveChan1 := make(chan []byte, 10)
		receiveChan2 := make(chan []byte, 10)
		firehoseWs1, _ := AddWSSink(receiveChan1, "4567", "/firehose/hose-subscription-1")
		firehoseWs2, _ := AddWSSink(receiveChan2, "4567", "/firehose/hose-subscription-1")
		defer firehoseWs1.Close()
		defer firehoseWs2.Close()

		for i := 0; i < 10; i++ {
			SendAppLog(appID, "message", inputConnection)
		}

		Eventually(func() int {
			return len(receiveChan1) + len(receiveChan2)
		}).Should(Equal(10))

		Expect(len(receiveChan1) - len(receiveChan2)).To(BeNumerically("~", 0, 2.5))
	})
})
