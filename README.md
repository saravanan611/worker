## 📚 **Overview**

`WorkerScall` is a **dynamic worker pool** for managing jobs concurrently. It automatically **scales the number of workers** based on the workload, ensuring efficient job processing without overwhelming your system.

* 🔥 **Dynamic Scaling**: Automatically add/remove workers based on the job queue size.
* 🚀 **High Efficiency**: Handles multiple jobs concurrently with minimal overhead.
* ⏱️ **Timeout Control**: Configurable timeouts for better job management.
* ⚙️ **Graceful Shutdown**: Ensures all jobs are completed or canceled before the pool stops.

---

## ⚡ **Key Features**

* **Dynamic Scaling**: Adjusts the number of workers as the queue grows or shrinks.
* **Job Progress Tracking**: Option to track the progress of each job.
* **Graceful Shutdown**: Stops the pool and waits for all jobs to finish.
* **Customizable Job Handling**: You define how jobs are processed.

---

## 🚀 **Getting Started**

### 1. **Install the Package**

Simply import the `worker` package into your project:

```go
import "github.com/saravanan611/worker"
```

### 2. **Create a Worker Pool**

Use `CreateScall` to initialize your worker pool. Here's how you can do it:

```go
package main

import (
    "fmt"
    "time"
    "github.com/saravanan611/worker"
)

func jobHandler(job string) string {
    time.Sleep(1 * time.Second)
    return fmt.Sprintf("Processed: %s", job)
}

func main() {
    // Create the worker pool
    workerPool, err := worker.CreateScall(
        5*time.Second,  // Scall cycle time
        2,              // Min workers
        10,             // Max workers
        100,            // Queue size
        20,             // Scall point (when to scale workers)
        jobHandler,     // Job handler function
        false,           // No progress tracking in this example
    )
    if err != nil {
        fmt.Println("Error creating worker pool:", err)
        return
    }

    // Add jobs to the pool
    for i := 0; i < 50; i++ {
        workerPool.Do(fmt.Sprintf("Job #%d", i))
    }

    // Gracefully stop the pool after jobs are processed
    workerPool.Stop()
}
```

---

## 🛠 **How It Works**

### 1. **Dynamic Scaling** ⚖️

* **Scaling Up**: When the number of jobs exceeds the threshold (`scallPoint`), more workers are added.
* **Scaling Down**: When the job queue shrinks, the number of workers decreases, ensuring optimal resource use.

### 2. **Job Processing** 🚀

* Jobs are processed by workers concurrently. Each worker fetches jobs from the queue and processes them using the `jobHandler` function you provide.
* You can add jobs to the queue using the `Do()` method.

### 3. **Graceful Shutdown** 🛑

* Call `Stop()` to gracefully shut down the worker pool once all jobs are processed.

---

## 📝 **Functions Overview**

### **CreateScall** 📅

```go
func CreateScall[pJob, pExpected any](pScallCycle time.Duration, pMin, pMax, pQSize, pScallPoint int, pFunc func(pJob) pExpected, pExpectedFlab bool) (lWorkerRec *WorkerScall[pJob, pExpected], lErr error)
```

* Creates a scalable worker pool.
* Arguments:

  * `pScallCycle`: Duration between scaling checks.
  * `pMin`, `pMax`: Min & max workers.
  * `pQSize`: Job queue size.
  * `pScallPoint`: Threshold for scaling workers.
  * `pFunc`: Function to process each job.
  * `pExpectedFlab`: Whether to track job progress.

---

### **Do** 📝

```go
func (pWorkerRec *WorkerScall[pJob, pExpected]) Do(pWork pJob)
```

* Adds a job to the worker pool. The job will be picked up by a worker for processing.

---

### **Stop** 🛑

```go
func (pWorkerRec *WorkerScall[pJob, pExpected]) Stop()
```

* Gracefully stops the worker pool after all jobs are processed.

---

## 🎨 **Customization & Advanced Use**

You can customize how each job is processed by providing your own **job handler function** to `CreateScall`. This allows you to define complex job logic for your application.

---

## 🧑‍💻 **Example Use Cases**

### **Use Case 1: Processing Network Requests**

If you're handling multiple TCP connections (like a web server), you can adapt this worker pool to process each connection concurrently, scaling workers up/down based on traffic.

### **Use Case 2: Batch Processing Jobs**

In a batch processing system (e.g., data processing, image resizing), you can scale workers to meet the demand for batch jobs and ensure each job is processed concurrently.

---

## 📈 **Benefits**

* **Efficient Resource Usage**: Dynamically scales workers to match the workload.
* **Simpler Code**: You don’t need to manually manage workers and queues.
* **Scalable**: Can handle large workloads without increasing complexity.
* **Graceful Shutdown**: Ensures no jobs are left unfinished.

---
Sure! Here's the **Working Principle Flowchart** to visualize how the `WorkerScall` package works. The flowchart explains how jobs are added, workers scale up and down, and how the worker pool processes tasks.

---

### 🏗️ **WorkerScall Flowchart**

```plaintext
        +------------------+
        |  CreateScall()   |
        |  Initialize Pool |
        +--------+---------+
                 |
                 v
     +--------------------------+
     | Add Jobs to the Pool     |
     | (Do() Method)             |
     +-----------+--------------+
                 |
                 v
       +--------------------+
       | Is There Space?     | 
       | (IsSpaceIn())       |
       +-----------+--------+
                 |
         +-------+-------+
         |               |
      No |               | Yes
         |               |
         v               v
  +---------------+  +-----------------+
  | Wait for Space |  | Job is Added to |
  | or Scaling Up  |  | Queue (job <-)   |
  +---------------+  +-----------------+
                 |
                 v
        +--------------------+
        | Job is Picked Up   |
        | by Worker (Worker) |
        +--------------------+
                 |
                 v
        +----------------------+
        | Execute Job with     |
        | jobHandler() function|
        +----------------------+
                 |
                 v
        +-------------------+
        | Job Processed      |
        | Return Result (if  |
        | Progress Flag set) |
        +-------------------+
                 |
                 v
        +---------------------+
        | Check for Scaling   |
        | (scallup() / scallDown()) |
        +---------------------+
                 |
       +---------+---------+
       |                   |
    No |                   | Yes
       |                   |
       v                   v
+-------------------+   +-----------------+
| Wait for Jobs to   |   | Scale Workers   |
| Finish             |   | (Increase/Decrease) |
+-------------------+   +-----------------+
                 |
                 v
        +---------------------+
        | Stop Worker Pool     |
        | (Stop())             |
        +---------------------+
```

---

### **Explanation of Flowchart**

1. **CreateScall**:

   * The process begins with calling `CreateScall()`, which initializes the worker pool with the given parameters (minimum workers, maximum workers, job queue size, etc.).

2. **Add Jobs to the Pool**:

   * Jobs are added to the pool using the `Do()` method. This places jobs into the job queue.

3. **Is There Space? (IsSpaceIn)**:

   * The system checks if there is space in the job queue. If there’s no space, the pool waits until space becomes available or scales up.

4. **Job Added to Queue**:

   * If there’s space, the job is added to the queue and will be picked up by a worker.

5. **Job Picked Up by Worker**:

   * Workers start picking jobs from the queue and process them.

6. **Execute Job**:

   * Each worker executes the job using the `jobHandler()` function provided at initialization.

7. **Job Processed**:

   * Once the job is processed, if the progress tracking flag is enabled, results are sent to the progress channel.

8. **Check for Scaling**:

   * After each job is completed, the system checks if the number of workers needs to be scaled up or down based on the queue size (`scallup()`/`scallDown()`).

9. **Scale Workers**:

   * If needed, workers are added or removed to optimize processing power.

10. **Wait for Jobs to Finish**:

    * If there are no jobs remaining or scaling is complete, the system waits for all workers to finish processing.

11. **Stop Worker Pool**:

    * Once all jobs are processed, the worker pool is stopped with the `Stop()` method, gracefully shutting down the pool.

---

### **Summary of Flow**

* Jobs are added to a queue.
* Workers process the jobs concurrently.
* The worker pool dynamically adjusts the number of workers based on the number of jobs in the queue (scaling up or down).
* The pool continues processing jobs until all tasks are completed, at which point it gracefully shuts down.



## 📑 **Conclusion**

The `WorkerScall` package is a simple, yet powerful solution for managing concurrent jobs with automatic scaling. It's perfect for handling varying workloads and ensures efficient resource management.

---

**Happy Coding!** 🚀👨‍💻

