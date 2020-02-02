#!/bin/bash
set -ex

. secrets/export_api_key.sh
. secrets/export_master_list_id.sh

go install

uncannifier
