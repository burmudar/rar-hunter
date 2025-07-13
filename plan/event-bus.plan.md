# RAR Hunter Event-Bus Architecture Migration Plan

## Overview

### Current State
RAR Hunter is a Go CLI tool that scans directories for RAR archives and extracts them automatically. The current architecture has several critical issues:

- **Uncontrolled concurrency**: Unbounded goroutine fan-out can overwhelm system resources
- **No cancellation**: Ctrl-C leaves child unrar processes running indefinitely  
- **Tight coupling**: Scanning, validation, and extraction logic are intertwined
- **Poor error handling**: Silent failures and inadequate error reporting
- **Limited testability**: Direct system calls make unit testing difficult

### Target Architecture
We're migrating to an event-driven architecture with these components:

```
Scanner → EventBus → Planner → EventBus → ProgramRunner
   ↓                    ↓                      ↓
DirFound           JobQueued              ProgramDone
```

#### Component Responsibilities

**Scanner**
- **Purpose**: Directory traversal and discovery
- **Responsibility**: Walk filesystem trees, identify directories that may contain RAR archives
- **Output**: Emits `DirFound` events for each discovered directory
- **Key Features**: Context-aware cancellation, configurable depth limits, concurrent directory scanning

**EventBus** 
- **Purpose**: Decoupled communication hub
- **Responsibility**: Route events between components, manage subscriptions, ensure type safety
- **Key Features**: Type-safe event handling, subscriber lifecycle management, event filtering
- **Pattern**: Publish-subscribe with optional event persistence for debugging

**Planner**
- **Purpose**: Work validation and job creation  
- **Responsibility**: Evaluate directories against criteria (SFV validation, missing files check), decide what work needs to be done
- **Input**: Subscribes to `DirFound` events
- **Output**: Emits `JobQueued` events for valid extraction targets
- **Key Features**: Pluggable criteria system, dry-run mode, duplicate detection

**ProgramRunner**
- **Purpose**: External process execution and lifecycle management
- **Responsibility**: Execute unrar commands with proper resource control, handle process lifecycle
- **Input**: Subscribes to `JobQueued` events  
- **Output**: Emits `ProgramStarted`/`ProgramDone` events with results
- **Key Features**: Worker pool concurrency control, timeout handling, graceful cancellation, process cleanup

#### Event Types

- **`DirFound`**: Contains directory path and metadata for potential processing
- **`JobQueued`**: Contains validated unrar job with target file, working directory, and options
- **`ProgramStarted`**: Indicates unrar process has begun, includes process ID and job details
- **`ProgramDone`**: Contains execution results, output, errors, and completion status

**Benefits:**
- **Loose coupling**: Components communicate only through events
- **Lifecycle management**: Context-based cancellation of all operations
- **Bounded concurrency**: Worker pools prevent resource exhaustion
- **Testability**: Interface-based design enables comprehensive testing
- **Extensibility**: New features (web UI, metrics) add as event subscribers

### Migration Approach
- **Incremental**: Each phase produces working, testable code
- **Backward compatible**: Existing CLI behavior preserved during transition
- **Risk mitigation**: Original code remains until new architecture is proven stable

---

## Phase 1: Foundation & Core Infrastructure (6 hours)

### Task 1.1: Create Event Bus Infrastructure (2 hours)
- [ ] Create `pkg/eventbus/` package
- [ ] Implement type-safe EventBus with subscription/publishing
- [ ] Add context cancellation support for subscribers
- [ ] Write basic unit tests for event bus functionality

### Task 1.2: Define Core Event Types (1 hour)
- [ ] Create `pkg/events/` package
- [ ] Define event types: `DirFound`, `JobQueued`, `ProgramStarted`, `ProgramDone`, `ScanComplete`
- [ ] Add common event interfaces and metadata (timestamps, correlation IDs)

### Task 1.3: Fix Data Model Issues (1 hour)
- [ ] Refactor `DirSnapshot` to store full relative paths instead of just basenames
- [ ] Fix filename collision issues in directory scanning
- [ ] Improve SFV parsing to handle multiple spaces/tabs properly

### Task 1.4: Add Context & Error Handling (2 hours)
- [ ] Replace all `exec.Command` with `exec.CommandContext`
- [ ] Add proper error wrapping throughout codebase
- [ ] Replace silent failures with logged errors using structured logging

## Phase 2: Component Separation (6 hours)

### Task 2.1: Create Scanner Component (2 hours)
- [ ] Create `pkg/scanner/` package
- [ ] Extract directory walking logic from main.go
- [ ] Emit `DirFound` events instead of returning slice
- [ ] Add context cancellation support for scanning operations

### Task 2.2: Create Planner Component (2 hours)  
- [ ] Create `pkg/planner/` package
- [ ] Move criteria evaluation logic from `FindUnrarable`
- [ ] Subscribe to `DirFound` events, emit `JobQueued` events
- [ ] Separate validation logic from selection logic

### Task 2.3: Refactor Criteria System (2 hours)
- [ ] Fix `CriteriaResult.String()` format string bug
- [ ] Improve error semantics in `AlreadyUnrared`
- [ ] Make criteria functions pure (no side effects)
- [ ] Add comprehensive criteria tests

## Phase 3: Program Runner Implementation (6 hours)

### Task 3.1: Create ProgramRunner Interface (2 hours)
- [ ] Create `pkg/runner/` package  
- [ ] Define `ProgramRunner` interface for testability
- [ ] Implement concrete runner with `exec.CommandContext`
- [ ] Add worker pool pattern with configurable concurrency

### Task 3.2: Fix Process Management Issues (2 hours)
- [ ] Replace unbounded goroutine fan-out with worker pool
- [ ] Fix pipe deadlock issues in `pipeReader`
- [ ] Properly handle stdout/stderr separation
- [ ] Add process timeout configuration

### Task 3.3: Add Lifecycle Management (2 hours)
- [ ] Implement graceful shutdown on context cancellation
- [ ] Add process cleanup and resource management
- [ ] Emit `ProgramStarted`/`ProgramDone` events
- [ ] Handle process failures and retries

## Phase 4: Integration & CLI Updates (4 hours)

### Task 4.1: Wire Components Together (2 hours)
- [ ] Update main.go to create and wire all components
- [ ] Add signal handling (SIGINT/SIGTERM) for graceful shutdown
- [ ] Replace direct function calls with event-driven flow
- [ ] Add configuration structure for component settings

### Task 4.2: Improve CLI Interface (2 hours)
- [ ] Add CLI flags: `--dry-run`, `--concurrency`, `--timeout`
- [ ] Implement proper exit codes (0=success, 1=partial errors, 2=invalid usage)
- [ ] Add progress reporting subscriber
- [ ] Improve error aggregation and reporting

## Phase 5: Testing & Quality (6 hours)

### Task 5.1: Add Component Tests (3 hours)
- [ ] Create mocks for ProgramRunner interface
- [ ] Add unit tests for Scanner component
- [ ] Add unit tests for Planner component  
- [ ] Add integration tests for event flow

### Task 5.2: Add End-to-End Tests (2 hours)
- [ ] Create test fixtures with sample RAR files
- [ ] Add CLI integration tests
- [ ] Test graceful shutdown scenarios
- [ ] Test error handling and recovery

### Task 5.3: Code Quality Improvements (1 hour)
- [ ] Add golangci-lint configuration
- [ ] Fix all linting issues
- [ ] Add race condition testing (`go test -race`)
- [ ] Update go.mod to use released Go version

## Phase 6: Documentation & Packaging (2 hours)

### Task 6.1: Update Documentation (1 hour)
- [ ] Update README with new architecture overview
- [ ] Add usage examples for new CLI flags
- [ ] Document event types and flow
- [ ] Add troubleshooting guide

### Task 6.2: Package Structure (1 hour)
- [ ] Organize packages: `/pkg/eventbus`, `/pkg/scanner`, `/pkg/planner`, `/pkg/runner`
- [ ] Add package documentation
- [ ] Ensure clean import dependencies
- [ ] Add CHANGELOG.md for migration notes

## Success Criteria

- [ ] All unrar processes are properly cancelled on Ctrl-C
- [ ] Concurrency is bounded and configurable  
- [ ] Components are decoupled and individually testable
- [ ] Error handling is comprehensive with proper logging
- [ ] CLI maintains backward compatibility while adding new features
- [ ] Test coverage > 80% for all new packages

## Migration Strategy

1. **Backward Compatibility**: Maintain existing CLI behavior during transition
2. **Incremental Testing**: Each phase should have working, testable code
3. **Feature Flags**: Use build tags or config to enable new architecture gradually  
4. **Rollback Plan**: Keep original implementation until new one is proven stable

## Estimated Total Time: 30 hours

Each task is designed to be completable in ~2 hours with clear deliverables and acceptance criteria.
