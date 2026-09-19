import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import client from '../api/client';
import { VM, ComplianceResult } from '../api/types';

const fetchVMs = async (): Promise<VM[]> => {
  const res = await client.get('/vms');
  return res.data;
};

const fetchCompliance = async (vmId: string): Promise<ComplianceResult[]> => {
  const res = await client.get(`/compliance/vms/${vmId}`);
  return res.data;
};

const Compliance: React.FC = () => {
  const { data: vms, isLoading: vmsLoading, error: vmsError } = useQuery({
    queryKey: ['vms'],
    queryFn: fetchVMs,
  });

  const [selectedVMId, setSelectedVMId] = useState('');

  const { data: complianceResults, isLoading: complianceLoading } = useQuery({
    queryKey: ['compliance', selectedVMId],
    queryFn: () => fetchCompliance(selectedVMId),
    enabled: !!selectedVMId,
  });

  if (vmsLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-slate-400 text-lg">Loading compliance data...</div>
      </div>
    );
  }

  if (vmsError) {
    return (
      <div className="bg-red-900/30 border border-red-500 text-red-200 px-4 py-3 rounded">
        Failed to load VMs
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-slate-100">Compliance</h1>

      <div className="bg-slate-800 p-4 rounded-lg border border-slate-700">
        <label className="block text-slate-300 text-sm font-medium mb-2">Select VM to check compliance</label>
        <select
          value={selectedVMId}
          onChange={(e) => setSelectedVMId(e.target.value)}
          className="w-full max-w-md px-3 py-2 bg-slate-700 border border-slate-600 rounded text-slate-100 focus:outline-none focus:border-blue-500"
        >
          <option value="">Choose a VM</option>
          {vms?.map((vm) => (
            <option key={vm.id} value={vm.id}>{vm.name}</option>
          ))}
        </select>
      </div>

      {selectedVMId && complianceLoading && (
        <div className="text-slate-400">Checking compliance...</div>
      )}

      {complianceResults && complianceResults.length > 0 && (
        <div className="space-y-4">
          {complianceResults.map((result, idx) => (
            <div key={idx} className="bg-slate-800 rounded-lg border border-slate-700 overflow-hidden">
              <div className={`px-4 py-3 border-b border-slate-700 flex items-center gap-3 ${
                result.passed ? 'bg-green-900/20' : 'bg-red-900/20'
              }`}>
                <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${
                  result.passed ? 'bg-green-900/50 text-green-300' : 'bg-red-900/50 text-red-300'
                }`}>
                  {result.passed ? 'PASSED' : 'FAILED'}
                </span>
                <span className="text-slate-100 font-medium">{result.check_type}</span>
                <span className="text-slate-400 text-sm ml-auto">
                  Checked: {new Date(result.checked_at).toLocaleString()}
                </span>
              </div>

              {!result.passed && result.violations.length > 0 && (
                <div className="p-4">
                  <h3 className="text-sm font-medium text-slate-300 mb-3">Violations</h3>
                  <div className="space-y-2">
                    {result.violations.map((violation, vIdx) => (
                      <div key={vIdx} className="bg-slate-700/50 p-3 rounded border border-slate-600">
                        <div className="flex items-start justify-between">
                          <div>
                            <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${
                              violation.severity === 'critical' ? 'bg-red-900/50 text-red-300' :
                              violation.severity === 'warning' ? 'bg-yellow-900/50 text-yellow-300' :
                              'bg-blue-900/50 text-blue-300'
                            }`}>
                              {violation.severity}
                            </span>
                            <span className="text-slate-100 font-medium ml-2">{violation.rule}</span>
                          </div>
                          {violation.correctable && (
                            <span className="text-xs bg-blue-900/50 text-blue-300 px-2 py-0.5 rounded">
                              Auto-correctable
                            </span>
                          )}
                        </div>
                        <div className="mt-2 text-sm text-slate-300">
                          <span className="text-slate-400">Field:</span> {violation.field}
                        </div>
                        <div className="mt-1 text-sm">
                          <span className="text-slate-400">Expected:</span>{' '}
                          <span className="text-green-400">{violation.expected}</span>
                        </div>
                        <div className="mt-1 text-sm">
                          <span className="text-slate-400">Actual:</span>{' '}
                          <span className="text-red-400">{violation.actual}</span>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {selectedVMId && complianceResults && complianceResults.length === 0 && (
        <div className="text-center py-8 text-slate-400">No compliance data available</div>
      )}
    </div>
  );
};

export default Compliance;
