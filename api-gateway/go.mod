module foodrush/api-gateway

go 1.26.1

require (
	foodrush/orders v0.0.0
	github.com/gonzalo-fch/PaymentsService v0.0.0
	github.com/jesus-acev/user-service v0.0.0
	github.com/mh0316/catalog v0.0.0
	google.golang.org/grpc v1.81.0
	google.golang.org/protobuf v1.36.11
)

require (
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
)

replace github.com/gonzalo-fch/PaymentsService => ../payment-service

replace github.com/jesus-acev/user-service => ../user-service

replace github.com/mh0316/catalog => ../catalog-service

replace foodrush/orders => ../order-service
