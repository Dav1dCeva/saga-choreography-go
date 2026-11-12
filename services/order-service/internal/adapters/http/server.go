// Package http adapta el caso de uso de pedidos a una API REST.
package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"saga-choreography-go/pkg/httpx"
	"saga-choreography-go/services/order-service/internal/application"
	"saga-choreography-go/services/order-service/internal/ports"
)

// Server expone los endpoints REST del servicio de pedidos.
type Server struct {
	createOrder *application.CreateOrder
	repository  ports.OrderRepository
	log         *slog.Logger
}

// NewServer construye el adaptador HTTP.
func NewServer(createOrder *application.CreateOrder, repository ports.OrderRepository, log *slog.Logger) *Server {
	return &Server{createOrder: createOrder, repository: repository, log: log}
}

// Register añade las rutas al mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /orders", s.create)
	mux.HandleFunc("GET /orders", s.list)
	mux.HandleFunc("GET /orders/{id}", s.get)
}

type createOrderRequest struct {
	Customer  string  `json:"customer"`
	Product   string  `json:"product"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unitPrice"`
	Address   string  `json:"address"`
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var request createOrderRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}

	order, err := s.createOrder.Execute(r.Context(), application.CreateOrderCommand{
		Customer:  strings.TrimSpace(request.Customer),
		Product:   strings.TrimSpace(request.Product),
		Quantity:  request.Quantity,
		UnitPrice: request.UnitPrice,
		Address:   strings.TrimSpace(request.Address),
	})
	if err != nil {
		if isValidationError(err) {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.log.Error("crear pedido", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "no se pudo crear el pedido")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, order)
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	order, err := s.repository.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "pedido no encontrado")
			return
		}
		s.log.Error("buscar pedido", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "no se pudo consultar el pedido")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, order)
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	orders, err := s.repository.List(r.Context())
	if err != nil {
		s.log.Error("listar pedidos", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "no se pudieron listar los pedidos")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, orders)
}

func isValidationError(err error) bool {
	return strings.Contains(err.Error(), "pedido inválido")
}
