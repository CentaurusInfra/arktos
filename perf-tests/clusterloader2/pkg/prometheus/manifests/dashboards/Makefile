# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

SOURCES := $(wildcard *.dashboard.py)
DASHBOARDS := $(SOURCES:%.dashboard.py=%.json)

init:
	pip install -r requirements.txt

%.json: %.dashboard.py defaults.py
 	# Setting PYTHONPATH to $PWD is a workaround for https://github.com/weaveworks/grafanalib/issues/58
	PYTHONPATH="$$PYTHONPATH:$(shell pwd)" generate-dashboard  "$<" | jq -M . > "$@"

all: $(DASHBOARDS)
