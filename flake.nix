{
  description = "Obsrvr Flow Plugin: Source BufferedStorage GCS";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  # Allow dirty Git working tree for development
  nixConfig = {
    allow-dirty = true;
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        
        # Create a custom Go derivation with fixed compiler version
        customGo = pkgs.go_1_23;
      in
      {
        packages = {
          default = pkgs.stdenv.mkDerivation {
            pname = "flow-source-bufferedstorage-gcs";
            version = "0.1.0";
            src = ./.;
            
            # Required for plugins
            dontStrip = true;
            
            nativeBuildInputs = [ 
              customGo 
              pkgs.pkg-config
            ];
            
            buildInputs = [ 
              # Add any required C library dependencies here if needed
            ];
            
            buildPhase = ''
              export GOCACHE=$TMPDIR/go-cache
              export GOPATH=$TMPDIR/go
              export CGO_ENABLED=1
              
              # Build as a plugin
              ${customGo}/bin/go build -mod=vendor -buildmode=plugin -o flow-source-bufferedstorage-gcs.so .
            '';

            installPhase = ''
              mkdir -p $out/lib
              cp flow-source-bufferedstorage-gcs.so $out/lib/
              
              # Also install a copy of go.mod for future reference
              mkdir -p $out/share
              cp go.mod $out/share/
              if [ -f go.sum ]; then
                cp go.sum $out/share/
              fi
            '';
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [ 
            go_1_23
            pkg-config
            git  # Needed for vendoring dependencies
            gopls
            delve
          ];
          
          # Shell setup for development environment
          shellHook = ''
            # Enable CGO which is required for plugin mode
            export CGO_ENABLED=1
            export GOFLAGS="-mod=vendor"
            
            # Helper to vendor dependencies - greatly improves build reliability
            if [ ! -d vendor ]; then
              echo "Vendoring dependencies..."
              go mod tidy
              go mod vendor
            fi
            
            echo "Development environment ready!"
            echo "To build the plugin manually: go build -buildmode=plugin -o flow-source-bufferedstorage-gcs.so ."
          '';
        };
      }
    );
} 