package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	paymentpb "github.com/gonzalo-fch/PaymentsService/pb"
	userpb "github.com/jesus-acev/user-service/pb"
	catalogpb "github.com/mh0316/catalog/pb"
	orderpb "foodrush/orders/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type gateway struct {
	usersConn    *grpc.ClientConn
	catalogConn  *grpc.ClientConn
	ordersConn   *grpc.ClientConn
	paymentsConn *grpc.ClientConn

	users    userpb.UsersServiceClient
	catalog  catalogpb.CatalogServiceClient
	orders   orderpb.OrderServiceClient
	payments paymentpb.PaymentsServiceClient
}

type errorResponse struct {
	Error string `json:"error"`
}

type statusResponse struct {
	Status string `json:"status"`
}

func main() {
	gw := mustNewGateway()
	defer gw.close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", gw.root)
	mux.HandleFunc("GET /healthz", gw.health)
	mux.HandleFunc("POST /users", gw.createUser)
	mux.HandleFunc("GET /users/{id}", gw.getUserProfile)
	mux.HandleFunc("GET /catalog/comercios", gw.listComercios)
	mux.HandleFunc("GET /catalog/comercios/{id}/menu", gw.getMenuByComercio)
	mux.HandleFunc("GET /catalog/products/{id}", gw.getProductDetails)
	mux.HandleFunc("POST /orders", gw.createOrder)
	mux.HandleFunc("GET /orders/{id}", gw.getOrderDetails)
	mux.HandleFunc("POST /orders/pickup/confirm", gw.confirmOrderPickup)
	mux.HandleFunc("POST /payments/process", gw.processPayment)
	mux.HandleFunc("GET /payments/order/{order_id}", gw.getPaymentByOrder)

	port := getEnv("API_GATEWAY_PORT", "8080")
	log.Printf("API Gateway listening on :%s", port)
	if err := http.ListenAndServe(":"+port, withJSONHeaders(mux)); err != nil {
		log.Fatalf("gateway stopped: %v", err)
	}
}

func mustNewGateway() *gateway {
	ctx := context.Background()

	usersConn := dialWithRetry(ctx, getEnv("USER_SERVICE_ADDR", "user-service:50051"))
	catalogConn := dialWithRetry(ctx, getEnv("CATALOG_SERVICE_ADDR", "catalog-service:50051"))
	ordersConn := dialWithRetry(ctx, getEnv("ORDERS_SERVICE_ADDR", "orders-service:50053"))
	paymentsConn := dialWithRetry(ctx, getEnv("PAYMENTS_SERVICE_ADDR", "payments-service:50051"))

	return &gateway{
		usersConn:    usersConn,
		catalogConn:  catalogConn,
		ordersConn:   ordersConn,
		paymentsConn: paymentsConn,
		users:        userpb.NewUsersServiceClient(usersConn),
		catalog:      catalogpb.NewCatalogServiceClient(catalogConn),
		orders:       orderpb.NewOrderServiceClient(ordersConn),
		payments:     paymentpb.NewPaymentsServiceClient(paymentsConn),
	}
}

func (g *gateway) close() {
	_ = g.usersConn.Close()
	_ = g.catalogConn.Close()
	_ = g.ordersConn.Close()
	_ = g.paymentsConn.Close()
}

func (g *gateway) root(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service": "api-gateway",
		"status": "ok",
		"routes": []string{
			"GET /healthz",
			"POST /users",
			"GET /users/{id}",
			"GET /catalog/comercios",
			"GET /catalog/comercios/{id}/menu",
			"GET /catalog/products/{id}",
			"POST /orders",
			"GET /orders/{id}",
			"POST /orders/pickup/confirm",
			"POST /payments/process",
			"GET /payments/order/{order_id}",
		},
	})
}

func (g *gateway) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

func (g *gateway) createUser(w http.ResponseWriter, r *http.Request) {
	var req userpb.CreateUserRequest
	if err := decodeProtoBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := g.users.CreateUser(ctx, &req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeProtoJSON(w, http.StatusCreated, resp)
}

func (g *gateway) getUserProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id es obligatorio")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := g.users.GetUserProfile(ctx, &userpb.GetUserProfileRequest{Id: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeProtoJSON(w, http.StatusOK, resp)
}

func (g *gateway) listComercios(w http.ResponseWriter, r *http.Request) {
	soloActivos := false
	if raw := r.URL.Query().Get("solo_activos"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "solo_activos debe ser true o false")
			return
		}
		soloActivos = value
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := g.catalog.ListComercios(ctx, &catalogpb.ListComerciosRequest{SoloActivos: soloActivos})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeProtoJSON(w, http.StatusOK, resp)
}

func (g *gateway) getMenuByComercio(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id es obligatorio")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := g.catalog.GetMenuByComercio(ctx, &catalogpb.GetMenuByComercioRequest{ComercioId: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeProtoJSON(w, http.StatusOK, resp)
}

func (g *gateway) getProductDetails(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id es obligatorio")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := g.catalog.GetProductDetails(ctx, &catalogpb.GetProductDetailsRequest{Id: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeProtoJSON(w, http.StatusOK, resp)
}

func (g *gateway) createOrder(w http.ResponseWriter, r *http.Request) {
	var req orderpb.CreateOrderRequest
	if err := decodeProtoBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := g.orders.CreateOrder(ctx, &req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeProtoJSON(w, http.StatusCreated, resp)
}

func (g *gateway) getOrderDetails(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id es obligatorio")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := g.orders.GetOrderDetails(ctx, &orderpb.GetOrderDetailsRequest{Id: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeProtoJSON(w, http.StatusOK, resp)
}

func (g *gateway) confirmOrderPickup(w http.ResponseWriter, r *http.Request) {
	var req orderpb.ConfirmOrderPickupRequest
	if err := decodeProtoBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := g.orders.ConfirmOrderPickup(ctx, &req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeProtoJSON(w, http.StatusOK, resp)
}

func (g *gateway) processPayment(w http.ResponseWriter, r *http.Request) {
	var req paymentpb.ProcessPaymentRequest
	if err := decodeProtoBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := g.payments.ProcessPayment(ctx, &req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeProtoJSON(w, http.StatusCreated, resp)
}

func (g *gateway) getPaymentByOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("order_id")
	if orderID == "" {
		writeError(w, http.StatusBadRequest, "order_id es obligatorio")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := g.payments.GetPaymentByOrder(ctx, &paymentpb.GetPaymentByOrderRequest{OrderId: orderID})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeProtoJSON(w, http.StatusOK, resp)
}

func dialWithRetry(ctx context.Context, addr string) *grpc.ClientConn {
	for i := 1; i <= 10; i++ {
		dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		conn, err := grpc.DialContext(
			dialCtx,
			addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		cancel()
		if err == nil {
			return conn
		}

		log.Printf("esperando servicio %s (%d/10): %v", addr, i, err)
		time.Sleep(2 * time.Second)
	}

	log.Fatalf("no se pudo conectar a %s", addr)
	return nil
}

func decodeProtoBody(r *http.Request, msg proto.Message) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return errors.New("body vacio")
	}

	return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(body, msg)
}

func writeProtoJSON(w http.ResponseWriter, statusCode int, msg proto.Message) {
	data, err := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}.Marshal(msg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo serializar la respuesta")
		return
	}

	w.WriteHeader(statusCode)
	_, _ = w.Write(data)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, errorResponse{Error: message})
}

func writeGRPCError(w http.ResponseWriter, err error) {
	code := grpcstatus.Code(err)
	switch code {
	case codes.InvalidArgument:
		writeError(w, http.StatusBadRequest, err.Error())
	case codes.NotFound:
		writeError(w, http.StatusNotFound, err.Error())
	case codes.AlreadyExists:
		writeError(w, http.StatusConflict, err.Error())
	case codes.Unauthenticated:
		writeError(w, http.StatusUnauthorized, err.Error())
	case codes.PermissionDenied:
		writeError(w, http.StatusForbidden, err.Error())
	case codes.FailedPrecondition:
		writeError(w, http.StatusPreconditionFailed, err.Error())
	case codes.Unavailable:
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case codes.DeadlineExceeded:
		writeError(w, http.StatusGatewayTimeout, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(v)
}

func withJSONHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
