import React from 'react';
import { useQuery } from '@tanstack/react-query';
import client from '../api/client';
import { StoragePool } from '../api/types';

const fetchStoragePools = async (): Promise<StoragePool[]> => {
  const res = await client.get('/storage-pools');
  return res.data;
};

const formatBytes = (bytes: number): string => {
  const gb = bytes / (1024 ** 3);
  return gb >= 1024 ? `${(gb / 1024).toFixed(1)} TB` : `${gb.toFixed(1)} GB`;
};

const Storage: React.FC = () => {
  const { data: pools, isLoading, error } = useQuery({
    queryKey: ['storage-pools'],
    queryFn: fetchStoragePools,
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-slate-400 text-lg">Loading storage pools...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-900/30 border border-red-500 text-red-200 px-4 py-3 rounded">
        Failed to load storage pools
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-slate-100">Storage Pools</h1>

      <div className="bg-slate-800 rounded-lg border border-slate-700 overflow-hidden">
        <table className="w-full">
          <thead className="bg-slate-700/50">
            <tr>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Name</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Path</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Total</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Used</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Usage</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-700">
            {pools?.map((pool) => {
              const usagePercent = (pool.used_bytes / pool.total_bytes) * 100;
              return (
                <tr key={pool.id} className="hover:bg-slate-700/30">
                  <td className="px-4 py-3 text-slate-100 font-medium">{pool.name}</td>
                  <td className="px-4 py-3 text-slate-300 font-mono text-sm">{pool.path}</td>
                  <td className="px-4 py-3 text-slate-300">{formatBytes(pool.total_bytes)}</td>
                  <td className="px-4 py-3 text-slate-300">{formatBytes(pool.used_bytes)}</td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <div className="flex-1 bg-slate-700 rounded-full h-2 w-24">
                        <div
                          className={`h-2 rounded-full ${
                            usagePercent > 90 ? 'bg-red-500' :
                            usagePercent > 70 ? 'bg-yellow-500' : 'bg-green-500'
                          }`}
                          style={{ width: `${Math.min(usagePercent, 100)}%` }}
                        />
                      </div>
                      <span className="text-slate-400 text-sm">{usagePercent.toFixed(0)}%</span>
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
        {pools?.length === 0 && (
          <div className="text-center py-8 text-slate-400">No storage pools found</div>
        )}
      </div>
    </div>
  );
};

export default Storage;
