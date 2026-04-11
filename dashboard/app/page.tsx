"use client";

import { useState, useEffect, useRef } from "react";

const ADMIN_URL =
  process.env.NEXT_PUBLIC_GATEWAY_ADMIN_URL ?? "http://localhost:8081";

interface Metrics {
  total_requests: number;
  blocked_requests: number;
  last_execution_ns: number;
}

export default function Home() {
  const [file, setFile] = useState<File | null>(null);
  const [uploadStatus, setUploadStatus] = useState<
    "idle" | "uploading" | "success" | "error"
  >("idle");
  const [uploadMessage, setUploadMessage] = useState("");
  const [metrics, setMetrics] = useState<Metrics | null>(null);
  const [metricsError, setMetricsError] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const fetchMetrics = async () => {
      try {
        const res = await fetch(`${ADMIN_URL}/admin/metrics`);
        if (!res.ok) throw new Error();
        setMetrics(await res.json());
        setMetricsError(false);
      } catch {
        setMetricsError(true);
      }
    };

    fetchMetrics();
    const id = setInterval(fetchMetrics, 2000);
    return () => clearInterval(id);
  }, []);

  const handleUpload = async () => {
    if (!file) return;
    setUploadStatus("uploading");
    setUploadMessage("");

    const body = new FormData();
    body.append("plugin", file);

    try {
      const res = await fetch(`${ADMIN_URL}/admin/upload`, {
        method: "POST",
        body,
      });
      if (!res.ok) throw new Error((await res.text()) || "Upload failed");
      setUploadStatus("success");
      setUploadMessage("Plugin swapped successfully");
      setFile(null);
      if (fileInputRef.current) fileInputRef.current.value = "";
    } catch (err) {
      setUploadStatus("error");
      setUploadMessage(err instanceof Error ? err.message : "Upload failed");
    }
  };

  return (
    <main className="min-h-screen bg-gray-950 text-gray-100 p-8">
      <div className="max-w-4xl mx-auto">
        <h1 className="text-3xl font-bold mb-1">Aero Gateway</h1>
        <p className="text-gray-400 mb-8">Control panel</p>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Upload card */}
          <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
            <h2 className="text-lg font-semibold mb-1">Plugin Upload</h2>
            <p className="text-sm text-gray-400 mb-4">
              Hot-swap the active .wasm filter without restarting
            </p>

            <label className="block mb-3 cursor-pointer">
              <div
                className={`border-2 border-dashed rounded-lg p-6 text-center transition-colors ${
                  file
                    ? "border-blue-500 bg-blue-500/10"
                    : "border-gray-700 hover:border-gray-500"
                }`}
              >
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".wasm"
                  className="sr-only"
                  onChange={(e) => {
                    setFile(e.target.files?.[0] ?? null);
                    setUploadStatus("idle");
                    setUploadMessage("");
                  }}
                />
                {file ? (
                  <span className="text-blue-400 font-mono text-sm">
                    {file.name}
                  </span>
                ) : (
                  <span className="text-gray-500 text-sm">
                    Click to select a .wasm file
                  </span>
                )}
              </div>
            </label>

            <button
              onClick={handleUpload}
              disabled={!file || uploadStatus === "uploading"}
              className="w-full py-2 px-4 bg-blue-600 hover:bg-blue-500 disabled:bg-gray-700 disabled:text-gray-500 rounded-lg font-medium transition-colors"
            >
              {uploadStatus === "uploading" ? "Uploading..." : "Deploy Plugin"}
            </button>

            {uploadMessage && (
              <p
                className={`mt-3 text-sm ${
                  uploadStatus === "success" ? "text-green-400" : "text-red-400"
                }`}
              >
                {uploadMessage}
              </p>
            )}
          </div>

          {/* Metrics card */}
          <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
            <div className="flex items-center justify-between mb-1">
              <h2 className="text-lg font-semibold">Live Metrics</h2>
              <span
                className={`w-2 h-2 rounded-full ${
                  metricsError ? "bg-red-500" : "bg-green-500"
                }`}
              />
            </div>
            <p className="text-sm text-gray-400 mb-4">Refreshes every 2s</p>

            {metricsError ? (
              <p className="text-sm text-red-400">
                Cannot reach gateway — is it running?
              </p>
            ) : metrics === null ? (
              <p className="text-sm text-gray-500">Connecting...</p>
            ) : (
              <dl className="space-y-4">
                <MetricRow
                  label="Total Requests"
                  value={metrics.total_requests.toLocaleString()}
                />
                <MetricRow
                  label="Blocked Requests"
                  value={metrics.blocked_requests.toLocaleString()}
                  accent="text-red-400"
                />
                <MetricRow
                  label="Last Plugin Duration"
                  value={formatNs(metrics.last_execution_ns)}
                />
              </dl>
            )}
          </div>
        </div>
      </div>
    </main>
  );
}

function MetricRow({
  label,
  value,
  accent = "text-white",
}: {
  label: string;
  value: string;
  accent?: string;
}) {
  return (
    <div className="flex justify-between items-baseline border-b border-gray-800 pb-3">
      <dt className="text-sm text-gray-400">{label}</dt>
      <dd className={`text-xl font-mono font-semibold ${accent}`}>{value}</dd>
    </div>
  );
}

function formatNs(ns: number): string {
  if (ns === 0) return "—";
  if (ns < 1_000) return `${ns} ns`;
  if (ns < 1_000_000) return `${(ns / 1_000).toFixed(1)} µs`;
  return `${(ns / 1_000_000).toFixed(2)} ms`;
}
