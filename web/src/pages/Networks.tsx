import React from 'react';
import { useQuery } from '@tanstack/react-query';
import client from '../api/client';
import { Network } from '../api/types';

const fetchNetworks = async (): Promise<Network[]> => {
  const res = await client.get('/networks');
  return res.data;
};

const Networks: React.FC = () => {
  const { data: networks, isLoading, error } = useQuery({
    queryKey: ['networks'],
    queryFn: fetchNetworks,
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-slate-400 text-lg">Loading networks...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-900/30 border border-red-500 text-red-200 px-4 py-3 rounded">
        Failed to load networks
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-slate-100">Networks</h1>

      <div className="bg-slate-800 rounded-lg border border-slate-700 overflow-hidden">
        <table className="w-full">
          <thead className="bg-slate-700/50">
            <tr>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Name</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Bridge</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Subnet</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-700">
            {networks?.map((network) => (
              <tr key={network.id} className="hover:bg-slate-700/30">
                <td className="px-4 py-3 text-slate-100 font-medium">{network.name}</td>
                <td className="px-4 py-3 text-slate-300 font-mono text-sm">{network.bridge}</td>
                <td className="px-4 py-3 text-slate-300 font-mono text-sm">{network.subnet}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {networks?.length === 0 && (
          <div className="text-center py-8 text-slate-400">No networks found</div>
        )}
      </div>
    </div>
  );
};

export default Networks;
