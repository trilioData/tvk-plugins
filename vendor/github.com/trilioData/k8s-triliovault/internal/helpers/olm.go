package helpers

import (
	"context"
	"os"

	log "github.com/sirupsen/logrus"

	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/trilioData/k8s-triliovault/internal"
)

func SetAppScopeIfNotPresent(cl client.Client, namespace string) {
	log.Info("Setting up the APP_SCOPE environment")
	_, present := os.LookupEnv(internal.AppScope)
	if !present {
		opGroup := &olmv1.OperatorGroupList{}
		if err := cl.List(context.TODO(), opGroup, client.InNamespace(namespace)); err != nil {
			log.Errorf("Unable to get the operatorGroup %v", err)
			panic(err)
		}
		log.Infof("Target Namespace in operator group is %v", opGroup.Items[0].Spec.TargetNamespaces)
		if len(opGroup.Items[0].Spec.TargetNamespaces) == 0 {
			if err := os.Setenv(internal.AppScope, "Cluster"); err != nil {
				log.Errorf("Unable to set the APP_SCOPE env variable %v", err)
				panic(err)
			}
			log.Info("APP_SCOPE is set to Cluster")
		} else if opGroup.Items[0].Spec.TargetNamespaces[0] == namespace {
			if err := os.Setenv(internal.AppScope, "Namespace"); err != nil {
				log.Errorf("Unable to set the APP_SCOPE env variable %v", err)
				panic(err)
			}
			log.Info("APP_SCOPE is set to Namespaced")
		}
	}
}
