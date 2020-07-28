package schemafunc

import "k8s.io/apimachinery/pkg/runtime"

var AddSchemaFuncs []func(s *runtime.Scheme) error
