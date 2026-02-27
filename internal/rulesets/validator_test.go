package rulesets

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidations(t *testing.T) {
	tests := []struct {
		name   string
		rules  string
		errors []error
	}{
		{
			name:   "Simple Rule",
			rules:  "SecRule REQUEST_URI \"@contains /admin\" \"id:1,deny\"",
			errors: nil,
		},
		{
			name:   "None sense",
			rules:  "this makes no sense, now does it",
			errors: []error{fmt.Errorf("[1:0] Syntax error '[@0,0:3='this',<225>,1:0]': mismatched input 'this' expecting {<EOF>, QUOTE, '#', 'SecComponentSignature', 'SecServerSignature', 'SecWebAppId', 'SecCacheTransformations', 'SecChrootDir', 'SecConnEngine', 'SecHashEngine', 'SecHashKey', 'SecHashParam', 'SecHashMethodRx', 'SecHashMethodPm', 'SecContentInjection', 'SecArgumentSeparator', 'SecAuditLogStorageDir', 'SecAuditLogDirMode', 'SecAuditEngine', 'SecAuditLogFileMode', 'SecAuditLog2', 'SecAuditLog', 'SecAuditLogFormat', 'SecAuditLogParts', 'SecAuditLogRelevantStatus', 'SecAuditLogType', 'SecDebugLog', 'SecDebugLogLevel', 'SecGeoLookupDb', 'SecGsbLookupDb', 'SecGuardianLog', 'SecInterceptOnError', 'SecConnReadStateLimit', 'SecConnWriteStateLimit', 'SecSensorId', 'SecRuleInheritance', 'SecRulePerfTime', 'SecStreamInBodyInspection', 'SecStreamOutBodyInspection', 'SecPcreMatchLimit', 'SecPcreMatchLimitRecursion', 'SecArgumentsLimit', 'SecRequestBodyJsonDepthLimit', 'SecRequestBodyAccess', 'SecRequestBodyLimit', 'SecRequestBodyLimitAction', 'SecRequestBodyNoFilesLimit', 'SecResponseBodyAccess', 'SecResponseBodyLimit', 'SecResponseBodyLimitAction', 'SecRuleEngine', 'SecAction', 'SecDefaultAction', 'SecDisableBackendCompression', 'SecMarker', 'SecUnicodeMapFile', 'SecCollectionTimeout', 'SecHttpBlKey', 'SecRemoteRulesFailAction', CONFIG_SEC_RULE_REMOVE_BY_ID, 'SecRuleRemoveByMsg', 'SecRuleRemoveByTag', 'SecRuleUpdateTargetByTag', 'SecRuleUpdateTargetByMsg', 'SecRuleUpdateTargetById', 'SecRuleUpdateActionById', 'SecUploadKeepFiles', 'SecTmpSaveUploadedFiles', 'SecUploadDir', 'SecUploadFileLimit', 'SecUploadFileMode', 'SecXmlExternalEntity', 'SecResponseBodyMimeType', 'SecResponseBodyMimeTypesClear', 'SecCookieFormat', 'SecCookieV0Separator', 'SecDataDir', 'SecStatusEngine', 'SecTmpDir', 'SecRule', 'SecRuleScript', INT}")},
		},
		{
			name:   "Empty Rules",
			rules:  "",
			errors: nil,
		},
		{
			name:   "multi-line rules",
			rules:  "SecRule REQUEST_URI \"@contains /admin\" \"id:1,deny\"\nSecRule REQUEST_URI \"@contains /api\" \"id:2,deny\"",
			errors: nil,
		},
		{
			name:   "Unsupported @pmFromFile",
			rules:  "SecRule REQUEST_URI \"@pmFromFile %{DOCUMENT_ROOT}/whitelist.txt\" \"phase:1,id:1,allow\"",
			errors: []error{fmt.Errorf("[1:22] Unsupported operator: @pmFromFile")},
		},
		{
			name:   "Complex Rules",
			rules:  "SecRule REQUEST_FILENAME|REQUEST_COOKIES|!REQUEST_COOKIES:/__utm/|REQUEST_COOKIES_NAMES|ARGS_NAMES|ARGS|XML:/* \"@rx _(?:\\$\\$ND_FUNC\\$\\$_|_js_function)|(?:\\beval|new[\\s\\v]+Function[\\s\\v]*)\\(|String\\.fromCharCode|function\\(\\)\\{|this\\.constructor|module\\.exports=|\\([\\s\\v]*[^0-9A-Z_a-z]child_process[^0-9A-Z_a-z][\\s\\v]*\\)|process(?:\\.(?:(?:a(?:ccess|ppendfile|rgv|vailability)|c(?:aveats|h(?:mod|own)|(?:los|opyfil)e|p|reate(?:read|write)stream)|ex(?:ec(?:file)?|ists)|f(?:ch(?:mod|own)|data(?:sync)?|s(?:tat|ync)|utimes)|inodes|l(?:chmod|ink|stat|utimes)|mkd(?:ir|temp)|open(?:dir)?|r(?:e(?:ad(?:dir|file|link|v)?|name)|m)|s(?:pawn(?:file)?|tat|ymlink)|truncate|u(?:n(?:link|watchfile)|times)|w(?:atchfile|rite(?:file|v)?))(?:sync)?(?:\\.call)?\\(|binding|constructor|env|global|main(?:Module)?|process|require)|\\[[\\\"'`](?:(?:a(?:ccess|ppendfile|rgv|vailability)|c(?:aveats|h(?:mod|own)|(?:los|opyfil)e|p|reate(?:read|write)stream)|ex(?:ec(?:file)?|ists)|f(?:ch(?:mod|own)|data(?:sync)?|s(?:tat|ync)|utimes)|inodes|l(?:chmod|ink|stat|utimes)|mkd(?:ir|temp)|open(?:dir)?|r(?:e(?:ad(?:dir|file|link|v)?|name)|m)|s(?:pawn(?:file)?|tat|ymlink)|truncate|u(?:n(?:link|watchfile)|times)|w(?:atchfile|rite(?:file|v)?))(?:sync)?|binding|constructor|env|global|main(?:Module)?|process|require)[\\\"'`]\\])|(?:binding|constructor|env|global|main(?:Module)?|process|require)\\[|console(?:\\.(?:debug|error|info|trace|warn)(?:\\.call)?\\(|\\[[\\\"'`](?:debug|error|info|trace|warn)[\\\"'`]\\])|require(?:\\.(?:resolve(?:\\.call)?\\(|main|extensions|cache)|\\[[\\\"'`](?:(?:resolv|cach)e|main|extensions)[\\\"'`]\\])\" \\\n    \"id:934100,\\\n    phase:2,\\\n    block,\\\n    capture,\\\n    t:none,t:urlDecodeUni,t:jsDecode,t:removeWhitespace,t:base64Decode,t:urlDecodeUni,t:jsDecode,t:removeWhitespace,\\\n    msg:'Node.js Injection Attack 1/2',\\\n    logdata:'Matched Data: %{TX.0} found within %{MATCHED_VAR_NAME}: %{MATCHED_VAR}',\\\n    tag:'application-multi',\\\n    tag:'language-javascript',\\\n    tag:'platform-multi',\\\n    tag:'attack-rce',\\\n    tag:'attack-injection-generic',\\\n    tag:'paranoia-level/1',\\\n    tag:'OWASP_CRS',\\\n    tag:'capec/1000/152/242',\\\n    ver:'OWASP_CRS/4.0.1-dev',\\\n    severity:'CRITICAL',\\\n    multiMatch,\\\n    setvar:'tx.rce_score=+%{tx.critical_anomaly_score}',\\\n    setvar:'tx.inbound_anomaly_score_pl1=+%{tx.critical_anomaly_score}'\"\nSecRule REQUEST_COOKIES|!REQUEST_COOKIES:/__utm/|REQUEST_COOKIES_NAMES|ARGS_NAMES|ARGS|XML:/* \"@rx (?:__proto__|constructor\\s*(?:\\.|\\[)\\s*prototype)\" \\\n    \"id:934130,\\\n    phase:2,\\\n    block,\\\n    capture,\\\n    t:none,t:urlDecodeUni,t:jsDecode,\\\n    msg:'JavaScript Prototype Pollution',\\\n    logdata:'Matched Data: %{TX.0} found within %{MATCHED_VAR_NAME}: %{MATCHED_VAR}',\\\n    tag:'application-multi',\\\n    tag:'language-javascript',\\\n    tag:'platform-multi',\\\n    tag:'attack-rce',\\\n    tag:'attack-injection-generic',\\\n    tag:'paranoia-level/1',\\\n    tag:'OWASP_CRS',\\\n    tag:'capec/1/180/77',\\\n    ver:'OWASP_CRS/4.0.1-dev',\\\n    severity:'CRITICAL',\\\n    multiMatch,\\\n    setvar:'tx.rce_score=+%{tx.critical_anomaly_score}',\\\n    setvar:'tx.inbound_anomaly_score_pl1=+%{tx.critical_anomaly_score}'\"",
			errors: nil,
		},
		{
			name:   "Include",
			rules:  "Include @owasp_crs/*.conf\nSecRule REQUEST_URI \\\"@streq /admin\\\" \\\"id:101,phase:1,t:lowercase,deny\\\"",
			errors: []error{fmt.Errorf("[1:0] Syntax error '[@0,0:6='Include',<181>,1:0]': mismatched input 'Include' expecting {<EOF>, QUOTE, '#', 'SecComponentSignature', 'SecServerSignature', 'SecWebAppId', 'SecCacheTransformations', 'SecChrootDir', 'SecConnEngine', 'SecHashEngine', 'SecHashKey', 'SecHashParam', 'SecHashMethodRx', 'SecHashMethodPm', 'SecContentInjection', 'SecArgumentSeparator', 'SecAuditLogStorageDir', 'SecAuditLogDirMode', 'SecAuditEngine', 'SecAuditLogFileMode', 'SecAuditLog2', 'SecAuditLog', 'SecAuditLogFormat', 'SecAuditLogParts', 'SecAuditLogRelevantStatus', 'SecAuditLogType', 'SecDebugLog', 'SecDebugLogLevel', 'SecGeoLookupDb', 'SecGsbLookupDb', 'SecGuardianLog', 'SecInterceptOnError', 'SecConnReadStateLimit', 'SecConnWriteStateLimit', 'SecSensorId', 'SecRuleInheritance', 'SecRulePerfTime', 'SecStreamInBodyInspection', 'SecStreamOutBodyInspection', 'SecPcreMatchLimit', 'SecPcreMatchLimitRecursion', 'SecArgumentsLimit', 'SecRequestBodyJsonDepthLimit', 'SecRequestBodyAccess', 'SecRequestBodyLimit', 'SecRequestBodyLimitAction', 'SecRequestBodyNoFilesLimit', 'SecResponseBodyAccess', 'SecResponseBodyLimit', 'SecResponseBodyLimitAction', 'SecRuleEngine', 'SecAction', 'SecDefaultAction', 'SecDisableBackendCompression', 'SecMarker', 'SecUnicodeMapFile', 'SecCollectionTimeout', 'SecHttpBlKey', 'SecRemoteRulesFailAction', CONFIG_SEC_RULE_REMOVE_BY_ID, 'SecRuleRemoveByMsg', 'SecRuleRemoveByTag', 'SecRuleUpdateTargetByTag', 'SecRuleUpdateTargetByMsg', 'SecRuleUpdateTargetById', 'SecRuleUpdateActionById', 'SecUploadKeepFiles', 'SecTmpSaveUploadedFiles', 'SecUploadDir', 'SecUploadFileLimit', 'SecUploadFileMode', 'SecXmlExternalEntity', 'SecResponseBodyMimeType', 'SecResponseBodyMimeTypesClear', 'SecCookieFormat', 'SecCookieV0Separator', 'SecDataDir', 'SecStatusEngine', 'SecTmpDir', 'SecRule', 'SecRuleScript', INT}")},
		},
		{
			name:   "Plugin",
			rules:  "SecAction \n \"id:9503020,\n  phase:1,\n  nolog,\n  pass,\n  t:none,\n  ver:'body-decompress-plugin/1.0.0',\n  setvar:'tx.body-decompress-plugin_max_data_size_bytes=102400'\"",
			errors: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.errors, Validate(tt.rules))
		})
	}
}
