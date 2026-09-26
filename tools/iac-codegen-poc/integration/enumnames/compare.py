"""Compares the enum names of the handwritten provider with the generated
enum names (F54).

Usage: compare.py <provider checkout> <generated enums.go>...

It reads every map entry "name": pkg.CONSTANT in the provider's non-test Go
files, finds the constant in the generated maps, and prints each name that
differs. The exit status is 0 in every case: this is a report, not a test.
"""
import collections
import glob
import re
import sys

ENTRY = re.compile(r'^\s+"([A-Za-z0-9_ ]+)":\s+(\w+)\.([A-Z0-9_]+)(?:\.Ptr\(\))?,', re.M)
GENERATED = re.compile(r'^\s+"([a-z0-9_]+)":\s+\w+\.([A-Z0-9_]+),$', re.M)


def main(provider, generated_files):
    generated = {}
    for f in generated_files:
        for name, const in GENERATED.findall(open(f).read()):
            generated[const] = name
    counts = collections.Counter()
    differ = []
    for f in sorted(glob.glob(provider + '/internal/provider/**/*.go', recursive=True)):
        if f.endswith('_test.go'):
            continue
        for name, _, const in ENTRY.findall(open(f).read()):
            rule = generated.get(const)
            if rule is None:
                counts['not a generated enum constant'] += 1
            elif rule == name:
                counts['same name'] += 1
            elif rule.replace('_', '') == name.lower().replace('_', ''):
                counts['letter case only'] += 1
                differ.append((f[len(provider) + 1:], name, rule))
            else:
                counts['other words'] += 1
                differ.append((f[len(provider) + 1:], name, rule))
    for k, v in counts.most_common():
        print(f'{v:4} {k}')
    for f, name, rule in differ:
        print(f'  {f}: handwritten {name!r}, rule {rule!r}')


if __name__ == '__main__':
    main(sys.argv[1], sys.argv[2:])
