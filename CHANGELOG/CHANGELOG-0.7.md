This release is mainly for for scalability improvements, with some functionality stabilization and build/test toolchain improvements. 

**Scalability scale-out architecture:**
* Introduce the new scale-out architecture in addition to the existing scale-up design. The new architecture supports multiple partitions, with each partition supporting the existing scale-up configuration.
* Initial implementation of the scale-out architecture that:
   * Supports multiple tenant partitions and a single resource manager. Multiple resource manager will be supported in subsequent releases.
   * Passes 20K density scalability test with 2 tenant partitions and 1 resource manager.

**Performance improvements ported from Kubernetes 1.18:**
* API Server cache index improvement
* List and watch performance improvement
* Fix watcher memory leak upon closing
* Object serialization improvement
* Reduce node status update frequency and pod patch update frequency

**Upgrade to build/runtime golang 1.13 version**

**Test Improvements:**
* Make density test cases tenancy-aware
* Improve scalability test suite for optimized configurations and better logging and tracing
 
**Critical bug fixes:**
* Fix bugs about PV/PVC for multi-tenancy (#937)
* Fix concurrent map write issue at AggregateWatcher stop (#825)
* Fix memory leak in AggregatedWatcher (#787)
