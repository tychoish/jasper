========================================
``jasper`` -- Process Management Service
========================================

Overview
--------

Jasper is a library for managing groups of processes in the context of
test automation, and was originally developed in the context of the
`Evergreen Continuous Integration Platform
<https://github.com/evergreen-ci/evergreen>`_ at MongoDB. 

The `deciduosity
<https://github.com/deciduosity/>`_ fork updates Jasper to use go modules, and
integrate more clearly into the deciduosity platform. Over time, it may make
sense to move some of the MongoDB-specific components into separate packages
and interfaces. 

Jasper is available for use under the terms of the Apache License (v2).

Documentation
-------------

The core API documentation is in the `godoc
<https://godoc.org/github.com/deciduosity/jasper/>`_.

Until there's documentation of the REST and gRPC interfaces, you can use the
`rest interface declaration
<https://github.com/deciduosity/jasper/blob/master/remote/rest_service.go>`_
and the `proto file
<https://github.com/deciduosity/jasper/blob/master/jasper.proto>`_ as a guide.

Development
-----------

Please feel free to open issues or pull requests if you encounter an issue or
would like to add a feature to Jasper. 

Jasper includes a ``makefile`` to support testing and development workflows.
