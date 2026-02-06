package logging

// Standard field names for structured logging across all Vorn subsystems.
const (
	FieldJobID     = "job_id"
	FieldModelID   = "model_id"
	FieldRuntimeID = "runtime_id"
	FieldGPUIndex  = "gpu_index"
	FieldGPUUUID   = "gpu_uuid"
	FieldMode      = "mode"
	FieldStatus    = "status"
	FieldError     = "error"
	FieldDuration  = "duration_ms"
	FieldComponent = "component"
	FieldVersion   = "version"
	FieldAddr      = "addr"
)

// Audit event types.
const (
	AuditModelAdd        = "model.add"
	AuditModelVerify     = "model.verify"
	AuditModelVerifyFail = "model.verify_failed"
	AuditModelRemove     = "model.remove"
	AuditModelDecrypt    = "model.decrypt"
	AuditJobSubmit       = "job.submit"
	AuditJobStart        = "job.start"
	AuditJobComplete     = "job.complete"
	AuditJobFail         = "job.fail"
	AuditJobKill         = "job.kill"
	AuditRuntimeRegister = "runtime.register"
	AuditRuntimeRemove   = "runtime.remove"
	AuditKeyLoad         = "key.load"
	AuditKeyRotate       = "key.rotate"
	AuditAuthSuccess     = "auth.success"
	AuditAuthFailure     = "auth.failure"
	AuditPolicyViolation = "policy.violation"
	AuditGPUError        = "gpu.error"
)
