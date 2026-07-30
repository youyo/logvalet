package cli

var HealthHandler = healthHandler
var APIKeyAuthMiddlewareForTest = apiKeyAuthMiddleware

func ResolvedAuthModeForTest(c *McpCmd) string { return c.resolvedAuthMode() }

func APIKeyValueForTest(c *McpCmd) string { return c.apiKeyValue() }
