#!/usr/bin/env python3
# pylint: disable=missing-function-docstring,missing-module-docstring
"""
Generate Kubernetes ConfigMaps from OWASP CoreRuleSet rules.

This script processes the CoreRuleSet dependency and creates ConfigMaps
for each rule file that contains SecRule or SecAction directives. Individual rules
containing @pmFromFile directives can optionally be removed with warnings via
the --ignore-pmFromFile flag. Rules with specific IDs can also be excluded
via the --ignore-rules argument.
"""

import argparse
import subprocess
import sys
import re
from pathlib import Path
from typing import List, Tuple, Set


# Base rules configmap content (from config/samples/ruleset.yaml)
BASE_RULES_CONFIGMAP = """apiVersion: v1
kind: ConfigMap
metadata:
  name: base-rules
data:
  rules: |
    SecRuleEngine On
    SecRequestBodyAccess On
    SecResponseBodyAccess Off
    SecAuditLog /dev/stdout
    SecAuditLogFormat JSON
    SecAuditEngine RelevantOnly
    SecDefaultAction "phase:2,log,auditlog,deny,status:403"
    SecAction \\
     "id:900990,\\
     phase:1,\\
     pass,\\
     t:none,\\
     nolog,\\
     tag:'OWASP_CRS',\\
     ver:'OWASP_CRS/4.23.0',\\
     setvar:tx.crs_setup_version=4230"
"""


def run_command(cmd: List[str]) -> Tuple[int, str, str]:
    """Run a command and return exit code, stdout, and stderr."""
    result = subprocess.run(
        cmd,
        capture_output=True,
        text=True,
        check=False
    )
    return result.returncode, result.stdout.strip(), result.stderr.strip()


def get_module_directory() -> str:
    """Get the directory path of the coreruleset module."""
    print("Getting coreruleset module directory...", file=sys.stderr)
    returncode, stdout, stderr = run_command([
        "go", "list", "-m", "-f", "{{.Dir}}",
        "github.com/corazawaf/coraza-coreruleset/v4"
    ])

    if returncode != 0:
        print(f"ERROR: Failed to get module directory: {stderr}", file=sys.stderr)
        sys.exit(1)

    if not stdout:
        print("ERROR: Module directory not found. Make sure the dependency is installed.", file=sys.stderr)
        sys.exit(1)

    print(f"Found module at: {stdout}", file=sys.stderr)
    return stdout


def get_rule_files(module_dir: str) -> List[Path]:
    """Get all .conf files from the @owasp_crs directory."""
    rules_dir = Path(module_dir) / "rules" / "@owasp_crs"

    if not rules_dir.exists():
        print(f"ERROR: Rules directory not found: {rules_dir}", file=sys.stderr)
        sys.exit(1)

    conf_files = sorted(rules_dir.glob("*.conf"))
    print(f"Found {len(conf_files)} .conf files", file=sys.stderr)
    return conf_files


def extract_rule_id(rule_text: str) -> str:
    """Extract the ID from a SecRule."""
    # Look for id:NUMBER pattern
    match = re.search(r'id:(\d+)', rule_text)
    if match:
        return match.group(1)
    return "unknown"


def split_into_rules(content: str) -> List[str]:
    """
    Split file content into individual rules/directives.

    Returns a list of text blocks, where each block is either:
    - A SecRule, SecAction, or other Sec* directive (potentially multi-line with backslash continuations)
    - Comments or blank lines
    """
    lines = content.split('\n')
    blocks = []
    current_block = []
    in_multiline = False

    for line in lines:
        stripped = line.rstrip()

        # Check if this is a continuation of a multi-line rule
        if in_multiline:
            current_block.append(line)
            if not stripped.endswith('\\'):
                in_multiline = False
                blocks.append('\n'.join(current_block))
                current_block = []
        # Check if this starts a Sec* directive (SecRule, SecAction, SecMarker, etc.)
        elif stripped.startswith('Sec'):
            current_block = [line]
            if stripped.endswith('\\'):
                in_multiline = True
            else:
                blocks.append('\n'.join(current_block))
                current_block = []
        # Other directives, comments, or blank lines
        else:
            blocks.append(line)

    # Add any remaining block
    if current_block:
        blocks.append('\n'.join(current_block))

    return blocks


def process_file_content(file_path: Path, ignore_rule_ids: Set[str], ignore_pmfromfile: bool) -> Tuple[str, List[Tuple[str, str]]]:
    """
    Process a rule file and remove rules with @pmFromFile or rules with IDs in the ignore list.

    Args:
        file_path: Path to the rule file
        ignore_rule_ids: Set of rule IDs to ignore
        ignore_pmfromfile: Whether to ignore rules containing @pmFromFile

    Returns:
        Tuple of (processed_content, list_of_(removed_rule_id, reason))
    """
    try:
        content = file_path.read_text(encoding='utf-8', errors='ignore')
    except Exception as e:
        print(f"ERROR: Failed to read {file_path}: {e}", file=sys.stderr)
        return "", []

    # Check if file has any SecRule or SecAction
    if "SecRule" not in content and "SecAction" not in content:
        return "", []

    blocks = split_into_rules(content)
    filtered_blocks = []
    removed_rules = []

    for block in blocks:
        stripped_block = block.strip()
        # Check if this block is a Sec* directive (SecRule, SecAction, etc.)
        if stripped_block.startswith('Sec'):
            # Check for @pmFromFile (only relevant for SecRule) if ignore flag is set
            if ignore_pmfromfile and stripped_block.startswith('SecRule') and '@pmFromFile' in block:
                rule_id = extract_rule_id(block)
                removed_rules.append((rule_id, "@pmFromFile not supported"))
            else:
                # Check if this Sec* directive has an ID in the ignore list
                rule_id = extract_rule_id(block)
                if rule_id in ignore_rule_ids:
                    removed_rules.append((rule_id, "Rule ID in ignore list"))
                else:
                    filtered_blocks.append(block)
        else:
            # Comments, blank lines, etc.
            filtered_blocks.append(block)

    processed_content = '\n'.join(filtered_blocks)
    return processed_content, removed_rules


def generate_configmap_name(file_path: Path) -> str:
    """Generate ConfigMap name from filename."""
    # Remove .conf extension and convert to lowercase
    name = file_path.stem.lower()
    return name


def generate_configmap(file_path: Path, ignore_rule_ids: Set[str], ignore_pmfromfile: bool) -> Tuple[str, str, str]:
    """
    Generate a ConfigMap YAML for a rule file.

    Args:
        file_path: Path to the rule file
        ignore_rule_ids: Set of rule IDs to ignore
        ignore_pmfromfile: Whether to ignore rules containing @pmFromFile

    Returns:
        Tuple of (configmap_name, configmap_yaml, skip_reason)
        skip_reason will be empty string if not skipped
    """
    configmap_name = generate_configmap_name(file_path)

    processed_content, removed_rules = process_file_content(file_path, ignore_rule_ids, ignore_pmfromfile)

    # Log removed rules
    if removed_rules:
        print(f"  ⚠ WARNING: Ignored rules in {file_path.name}:", file=sys.stderr)
        for rule_id, reason in removed_rules:
            print(f"    - Rule ID: {rule_id} ({reason})", file=sys.stderr)

    if not processed_content.strip():
        return "", "", "No SecRule or SecAction directives found"

    # Indent the rules content for YAML
    indented_rules = "\n".join(f"    {line}" if line else "" for line in processed_content.splitlines())

    configmap = f"""apiVersion: v1
kind: ConfigMap
metadata:
  name: {configmap_name}
data:
  rules: |
{indented_rules}
"""

    return configmap_name, configmap, ""


def generate_ruleset(configmap_names: List[str], include_base_rules: bool = True) -> str:
    """Generate a RuleSet resource referencing all ConfigMaps."""
    rules_entries = []

    # Add base-rules as the first entry if requested
    if include_base_rules:
        rules_entries.append("""  - apiVersion: v1
    kind: ConfigMap
    name: base-rules""")

    for name in configmap_names:
        rules_entries.append(f"""  - apiVersion: v1
    kind: ConfigMap
    name: {name}""")

    rules_section = '\n'.join(rules_entries)

    ruleset = f"""apiVersion: waf.k8s.coraza.io/v1alpha1
kind: RuleSet
metadata:
  name: default-ruleset
spec:
  rules:
{rules_section}
"""

    return ruleset


def main():
    # Parse command-line arguments
    parser = argparse.ArgumentParser(
        description='Generate Kubernetes ConfigMaps from OWASP CoreRuleSet rules',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Generate ConfigMaps without ignoring any rules
  %(prog)s

  # Ignore rules containing @pmFromFile directives
  %(prog)s --ignore-pmFromFile

  # Ignore specific rule IDs
  %(prog)s --ignore-rules 949110,949111,980130

  # Ignore rules using comma-separated list and @pmFromFile
  %(prog)s --ignore-rules "949110, 949111, 980130" --ignore-pmFromFile
"""
    )
    parser.add_argument(
        '--ignore-rules',
        type=str,
        default='',
        help='Comma-separated list of rule IDs to ignore (e.g., "949110,949111,980130")'
    )
    parser.add_argument(
        '--ignore-pmFromFile',
        action='store_true',
        help='Ignore rules containing @pmFromFile directives (not supported by Coraza)'
    )
    args = parser.parse_args()

    # Parse ignore list
    ignore_rule_ids: Set[str] = set()
    if args.ignore_rules:
        ignore_rule_ids = {rid.strip() for rid in args.ignore_rules.split(',') if rid.strip()}
        if ignore_rule_ids:
            print(f"Ignoring rule IDs: {', '.join(sorted(ignore_rule_ids))}", file=sys.stderr)

    # Get module directory
    module_dir = get_module_directory()

    # Get all rule files
    rule_files = get_rule_files(module_dir)

    # Process each file
    processed_count = 0
    skipped_count = 0
    configmaps = []
    configmap_names = []

    print(f"\nProcessing {len(rule_files)} files...\n", file=sys.stderr)

    for rule_file in rule_files:
        print(f"Processing: {rule_file.name}", file=sys.stderr)
        configmap_name, configmap_yaml, skip_reason = generate_configmap(rule_file, ignore_rule_ids, args.ignore_pmFromFile)

        if configmap_yaml:
            configmaps.append(configmap_yaml)
            configmap_names.append(configmap_name)
            processed_count += 1
            print(f"  ✓ Generated ConfigMap: {configmap_name}", file=sys.stderr)
        else:
            print(f"  ✗ Skipped: {skip_reason}", file=sys.stderr)
            skipped_count += 1

    # Generate RuleSet
    ruleset = generate_ruleset(configmap_names)

    # Output summary
    print(f"\n{'='*60}", file=sys.stderr)
    print(f"Summary:", file=sys.stderr)
    print(f"  Base rules: 1 (bundled)", file=sys.stderr)
    print(f"  Processed: {processed_count} files", file=sys.stderr)
    print(f"  Skipped: {skipped_count} files", file=sys.stderr)
    print(f"  Total ConfigMaps: {len(configmap_names) + 1}", file=sys.stderr)
    print(f"{'='*60}\n", file=sys.stderr)

    # Print base-rules ConfigMap first
    print(BASE_RULES_CONFIGMAP, end="")

    # Print generated ConfigMaps
    for configmap in configmaps:
        print("---")
        print(configmap, end="")

    # Print RuleSet
    print("---")
    print(ruleset, end="")


if __name__ == "__main__":
    main()
