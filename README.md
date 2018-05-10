# knetblackmon

_knetblackmon_ is a _kubernetes_ controller to do some blackbox monitoring on network overlay.

## Goal

Test your network overlay, between pods and services.

## Design

A controller in a  _daemonset_ is deployed on each node.

A service pointing to address all containers.

### endpoint scraper

Test node-to-node communication.

A pod on a node should be able to talk to pods on any other node.

### service scraper

Test pod-to-service communication.

A pod on a node should be able to talk to any service.
