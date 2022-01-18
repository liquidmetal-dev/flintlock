package metrics

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/ports"
	"github.com/weaveworks/flintlock/internal/command/flags"
	"github.com/weaveworks/flintlock/internal/config"
	"github.com/weaveworks/flintlock/internal/inject"
)

func serveCommand() *cli.Command {
	cfg := &config.Config{}

	return &cli.Command{
		Name:    "serve",
		Aliases: []string{"s"},
		Usage:   "Listen and serve HTTP.",
		Before:  flags.ParseFlags(cfg),
		Flags: flags.CLIFlags(
			flags.WithContainerDFlags(),
			flags.WithHTTPEndpointFlags(),
			flags.WithGlobalConfigFlags(),
		),
		Action: func(c *cli.Context) error {
			return serve(cfg)
		},
	}
}

func serve(cfg *config.Config) error {
	aports, err := inject.InitializePorts(cfg)
	if err != nil {
		return fmt.Errorf("initialising ports for application: %w", err)
	}

	router := mux.NewRouter()

	router.HandleFunc(
		"/machine/uid/{uid}",
		func(response http.ResponseWriter, request *http.Request) {
			vars := mux.Vars(request)

			vm, err := aports.Repo.Get(context.Background(), ports.RepositoryGetOptions{
				UID: vars["uid"],
			})
			if err != nil {
				logrus.Error(err.Error())
				response.WriteHeader(http.StatusInternalServerError)

				return
			}

			metrics, err := aports.Provider.Metrics(context.Background(), vm.ID)
			if err != nil {
				logrus.Error(err.Error())
				response.WriteHeader(http.StatusInternalServerError)

				return
			}

			response.WriteHeader(http.StatusOK)

			_, _ = response.Write(metrics.ToPrometheus())
		},
	)

	router.HandleFunc(
		"/machine/{namespace}/{name}",
		func(response http.ResponseWriter, request *http.Request) {
			vars := mux.Vars(request)

			mms, err := getAllMachineMetrics(
				context.Background(),
				aports,
				models.ListMicroVMQuery{
					"namespace": vars["namespace"],
					"name":      vars["name"],
				},
			)
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				_, _ = response.Write([]byte(err.Error()))

				return
			}

			response.WriteHeader(http.StatusOK)

			for _, mm := range mms {
				_, _ = response.Write(append(mm.ToPrometheus(), '\n'))
			}
		},
	)

	router.HandleFunc(
		"/machine/{namespace}",
		func(response http.ResponseWriter, request *http.Request) {
			vars := mux.Vars(request)

			mms, err := getAllMachineMetrics(context.Background(), aports, models.ListMicroVMQuery{"namespace": vars["namespace"]})
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				_, _ = response.Write([]byte(err.Error()))

				return
			}

			response.WriteHeader(http.StatusOK)

			for _, mm := range mms {
				_, _ = response.Write(append(mm.ToPrometheus(), '\n'))
			}
		},
	)

	router.HandleFunc(
		"/machine",
		func(response http.ResponseWriter, request *http.Request) {
			mms, err := getAllMachineMetrics(context.Background(), aports, models.ListMicroVMQuery{})
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				_, _ = response.Write([]byte(err.Error()))

				return
			}

			response.WriteHeader(http.StatusOK)

			for _, mm := range mms {
				_, _ = response.Write(append(mm.ToPrometheus(), '\n'))
			}
		},
	)

	logrus.Infof("Start listening on %s", cfg.HTTPAPIEndpoint)

	return http.ListenAndServe(cfg.HTTPAPIEndpoint, router)
}

func getAllMachineMetrics(ctx context.Context, aports *ports.Collection, query models.ListMicroVMQuery) ([]ports.MachineMetrics, error) {
	mms := []ports.MachineMetrics{}

	machines, err := aports.Repo.GetAll(ctx, query)
	if err != nil {
		return mms, err
	}

	for _, machine := range machines {
		metrics, err := aports.Provider.Metrics(ctx, machine.ID)
		if err != nil {
			return mms, err
		}

		mms = append(mms, metrics)
	}

	return mms, nil
}
