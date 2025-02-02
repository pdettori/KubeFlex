# typed: false
# frozen_string_literal: true

# This file was generated by GoReleaser. DO NOT EDIT.
class Kubeflex < Formula
  desc ""
  homepage "https://github.com/kubestellar/kubeflex"
  version "0.7.2"

  on_macos do
    on_intel do
      url "https://github.com/kubestellar/kubeflex/releases/download/v0.7.2/kubeflex_0.7.2_darwin_amd64.tar.gz"
      sha256 "e6bfd1d931f54d676090441ee6f564ffab19e00b39c0058dea4ab7f7ee50045f"

      def install
        bin.install "bin/kflex"
      end
    end
    on_arm do
      url "https://github.com/kubestellar/kubeflex/releases/download/v0.7.2/kubeflex_0.7.2_darwin_arm64.tar.gz"
      sha256 "eba11fc2a0d1fbcc0b412cc4962836678368f860c83bb0620e215d55b12f38e6"

      def install
        bin.install "bin/kflex"
      end
    end
  end

  on_linux do
    on_intel do
      if Hardware::CPU.is_64_bit?
        url "https://github.com/kubestellar/kubeflex/releases/download/v0.7.2/kubeflex_0.7.2_linux_amd64.tar.gz"
        sha256 "7af4dca22ba2090d787f9cbbf161ac1b9994d3928849a97d67fef40ab190d02e"

        def install
          bin.install "bin/kflex"
        end
      end
    end
    on_arm do
      if Hardware::CPU.is_64_bit?
        url "https://github.com/kubestellar/kubeflex/releases/download/v0.7.2/kubeflex_0.7.2_linux_arm64.tar.gz"
        sha256 "47fcfafb8cc135a770489071fbbd5843899f1134744d91854b82746ce17be09a"

        def install
          bin.install "bin/kflex"
        end
      end
    end
  end
end
