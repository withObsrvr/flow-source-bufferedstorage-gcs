{
  description = "Obsrvr Flow Plugin: Source BufferedStorage GCS";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages = {
          default = pkgs.buildGoModule {
            pname = "flow-source-bufferedstorage-gcs";
            version = "0.1.0";
            src = ./.;
            
            # Use null to skip vendoring check since we're using a vendor directory
            vendorHash = null;
            
            # Disable hardening which is required for Go plugins
            hardeningDisable = [ "all" ];
            
            # Configure build environment for plugin compilation 
            preBuild = ''
              export CGO_ENABLED=1
              # Use Go 1.23.4 to match go.mod exactly
              export GOFLAGS="-mod=vendor"
            '';
            
            # Build as a shared library/plugin
            buildPhase = ''
              runHook preBuild
              go build -mod=vendor -buildmode=plugin -o flow-source-bufferedstorage-gcs.so .
              runHook postBuild
            '';

            # Custom install phase for the plugin
            installPhase = ''
              runHook preInstall
              mkdir -p $out/lib
              cp flow-source-bufferedstorage-gcs.so $out/lib/
              # Also install a copy of go.mod for future reference
              mkdir -p $out/share
              cp go.mod $out/share/
              if [ -f go.sum ]; then
                cp go.sum $out/share/
              fi
              runHook postInstall
            '';
            
            # Add dependencies needed for the build
            nativeBuildInputs = [ pkgs.pkg-config ];
            buildInputs = [ 
              # Add any required C library dependencies here if needed
            ];
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [ 
            # Use Go 1.23 to match your project's go.mod
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