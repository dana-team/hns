# HNS: Code Structure
In this section, it is explained what the code structure of the `HNS` project is.

| Name                     | Explanation |
| ---                      | --- |
| `/api`                   | The API definitions |
| `/config`                | Different yaml and `kustomize` files for deployments |
| `/hack`                  | Contains boilerplate code |
| `/internals/controllers` | Contains all the reconcilers code and testing  |
| `/internals/webhooks`    | Contains all webhooks code |
| `/internals/utils`       | Contains all utility functions |
| `/test/e2e`              | Contains e2e tests |
| `/pkg/testutils`         | Contains utilities for testing |
| `/docs`                  | Contains doc files |