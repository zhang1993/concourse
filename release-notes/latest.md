#### <sub><sup><a name="5659" href="#5659">:link:</a></sup></sub> feature

* When [distributed tracing](https://concourse-ci.org/tracing.html) is configured, Concourse will now emit spans for several of its backend operations, including resource scanning, check execution, and job scheduling. These spans will be appropriately linked when viewed in a tracing tool like Jaeger, allowing operators to better observe the events that occur between resource checking and build execution. #5659

#### <sub><sup><a name="5653" href="#5653">:link:</a></sup></sub> feature

* When [distributed tracing](https://concourse-ci.org/tracing.html) is configured, all check, get, put, and task containers will be run with the `TRACEPARENT` environment variable set, which contains information about the parent span following the [w3c trace context format](https://www.w3.org/TR/trace-context/#traceparent-header):

  ```
  TRACEPARENT=version-trace_id-parent_id-trace_flags
  ```

  Using this information, your tasks and custom `resource_types` can emit spans to a tracing backend, and these spans will be appropriately linked to the step in which they ran. This can be particularly useful when integrating with downstream services that also support tracing. #5653
