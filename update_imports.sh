#!/bin/bash

# Script to update imports from gorat/common to gorat/pkg/protocol

FILES=$(find . -name "*.go" -exec grep -l "gorat/common" {} \;)

for file in $FILES; do
    echo "Updating $file"
    # Replace the import statement
    sed -i '' 's|"gorat/common"|"gorat/pkg/protocol"|g' "$file"
    # Replace common. references with protocol.
    sed -i '' 's|common\.|protocol.|g' "$file"
done

echo "Done updating imports!"