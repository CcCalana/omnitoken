#!/usr/bin/env python3
"""Load .env then run e2e_verify.py"""
import os
import sys

# Load .env
env_path = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), '.env')
if os.path.exists(env_path):
    with open(env_path, 'r', encoding='utf-8') as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith('#'):
                continue
            if '=' in line:
                key, val = line.split('=', 1)
                key = key.strip()
                val = val.strip()
                if not os.environ.get(key):
                    os.environ[key] = val

# Report what we loaded (safe)
bt = os.environ.get('OMNITOKEN_ADMIN_BOOTSTRAP_TOKEN', '')
if bt in ('', '<set-me-or-disable>'):
    bt = ''
    os.environ['OMNITOKEN_ADMIN_BOOTSTRAP_TOKEN'] = ''
ak = os.environ.get('OMNITOKEN_ARK_API_KEY', '')
print(f"Loaded .env: bootstrap_token={'set(len='+str(len(bt))+')' if bt else 'NOT SET'}, ark_key={'set(len='+str(len(ak))+')' if ak else 'NOT SET'}")
print()

# Now exec the main script
exec(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), 'e2e_verify.py'), encoding='utf-8').read())
