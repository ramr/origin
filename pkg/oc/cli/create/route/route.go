package route

import (
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/golang/glog"
	routev1 "github.com/openshift/api/route/v1"
)

// UnsecuredRoute will return a route with enough info so that it can direct traffic to
// the service provided by --service. Callers of this helper are responsible for providing
// tls configuration, path, and the hostname of the route.
// forcePort always sets a port, even when there is only one and it has no name.
// The kubernetes generator, when no port is present incorrectly selects the service Port
// instead of the service TargetPort for the route TargetPort.
func UnsecuredRoute(kc kclientset.Interface, namespace, routeName, serviceName, portString string, forcePort bool) (*routev1.Route, error) {
	if len(routeName) == 0 {
		routeName = serviceName
	}

	svc, err := kc.Core().Services(namespace).Get(serviceName, metav1.GetOptions{})
	if err != nil {
		glog.V(4).Infof("error getting service %q in namespace %q: %v", serviceName, namespace, err)

		if len(portString) == 0 {
			return nil, fmt.Errorf("you need to provide a route port via --port when exposing a non-existent service")
		}
		return &routev1.Route{
			// this is ok because we know exactly how we want to be serialized
			TypeMeta: metav1.TypeMeta{APIVersion: routev1.SchemeGroupVersion.String(), Kind: "Route"},
			ObjectMeta: metav1.ObjectMeta{
				Name: routeName,
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Name: serviceName,
				},
				Port: resolveRoutePort(portString),
			},
		}, nil
	}

	ok, port := supportsTCP(svc)
	if !ok {
		return nil, fmt.Errorf("service %q doesn't support TCP", svc.Name)
	}

	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:   routeName,
			Labels: svc.Labels,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Name: serviceName,
			},
		},
	}

	// When route.Spec.Port is not set, the generator will pick a service port.

	// If the user didn't specify --port, and either the service has a port.Name
	// or forcePort is set, set route.Spec.Port
	if (len(port.Name) > 0 || forcePort) && len(portString) == 0 {
		if len(port.Name) == 0 {
			route.Spec.Port = resolveRoutePort(svc.Spec.Ports[0].TargetPort.String())
		} else {
			route.Spec.Port = resolveRoutePort(port.Name)
		}
	}
	// --port uber alles
	if len(portString) > 0 {
		route.Spec.Port = resolveRoutePort(portString)
	}

	return route, nil
}

func resolveRoutePort(portString string) *routev1.RoutePort {
	if len(portString) == 0 {
		return nil
	}
	var routePort intstr.IntOrString
	integer, err := strconv.Atoi(portString)
	if err != nil {
		routePort = intstr.FromString(portString)
	} else {
		routePort = intstr.FromInt(integer)
	}
	return &routev1.RoutePort{
		TargetPort: routePort,
	}
}

func supportsTCP(svc *kapi.Service) (bool, kapi.ServicePort) {
	for _, port := range svc.Spec.Ports {
		if port.Protocol == kapi.ProtocolTCP {
			return true, port
		}
	}
	return false, kapi.ServicePort{}
}
