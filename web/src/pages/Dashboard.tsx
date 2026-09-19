import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import client from '../api/client';
import { Host, VM, Event } from '../api/types';

const fetchHosts = async (): Promise<Host[]> => {
  const res = await client.get('/hosts');
  return res.data;
};

const fetchVMs = async (): Promise<VM[]> => {
  const res = await client.get('/vms');
  return res.data;
};

const fetchEvents = async (): Promise<Event[]> => {
  const res = await client.get('/events');
  return res.data;
};

const formatBytes = (bytes: number): string => {
  const gb = bytes / (1024 ** 3);
  return gb >= 1024 ? `${(gb / 1024).toFixed(1)} TB` : `${gb.toFixed(1)} GB`;
};

const StatCard: React.FC<{ title: string; value: string | number; link: string }> = ({ title, value, link }) => (
  <Link to={link} className="bg-slate-800 p-6 rounded-lg border border-slate-700 hover:border-blue-500 transition-colors">
    <h3 className="text-slate-400 text-sm font-medium">{title}</h3>
    <p className="text-3xl font-bold text-slate-100 mt-2">{value}</p>
  </Link>
);

const Dashboard: React.FC = () => {
  const { data: hosts, isLoading: hostsLoading, error: hostsError } = useQuery({
    queryKey: ['hosts'],
    queryFn: fetchHosts,
  });

  const { data: vms, isLoading: vmsLoading, error: vmsError } = useQuery({
    queryKey: ['vms'],
    queryFn: fetchVMs,
  });

  const { data: events, isLoading: eventsLoading } = useQuery({
    queryKey: ['events'],
    queryFn: fetchEvents,
  });

  if (hostsLoading || vmsLoading || eventsLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-slate-400 text-lg">Loading dashboard...</div>
      </div>
    );
  }

  if (hostsError || vmsError) {
    return (
      <div className="bg-red-900/30 border border-red-500 text-red-200 px-4 py-3 rounded">
        Failed to load dashboard data
      </div>
    );
  }

  const runningVMs = vms?.filter((vm: VM) => vm.status === 'running').length ?? 0;
  const totalMemory = hosts?.reduce((sum: number, h: Host) => sum + h.memory_bytes, 0) ?? 0;
  const totalCPUs = hosts?.reduce((sum: number, h: Host) => sum + h.cpus, 0) ?? 0;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-slate-100">Dashboard</h1>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard title="Hosts" value={hosts?.length ?? 0} link="/hosts" />
        <StatCard title="Total VMs" value={vms?.length ?? 0} link="/vms" />
        <StatCard title="Running VMs" value={runningVMs} link="/vms" />
        <StatCard title="Recent Events" value={events?.slice(0, 5).length ?? 0} link="/events" />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-slate-800 p-6 rounded-lg border border-slate-700">
          <h2 className="text-lg font-semibold text-slate-100 mb-4">Resource Summary</h2>
          <div className="space-y-3">
            <div className="flex justify-between text-slate-300">
              <span>Total CPU Cores</span>
              <span className="font-medium">{totalCPUs}</span>
            </div>
            <div className="flex justify-between text-slate-300">
              <span>Total Memory</span>
              <span className="font-medium">{formatBytes(totalMemory)}</span>
            </div>
            <div className="flex justify-between text-slate-300">
              <span>Host Status</span>
              <span className="font-medium text-green-400">
                {hosts?.filter((h: Host) => h.status === 'online').length ?? 0} online
              </span>
            </div>
          </div>
        </div>

        <div className="bg-slate-800 p-6 rounded-lg border border-slate-700">
          <h2 className="text-lg font-semibold text-slate-100 mb-4">Recent Events</h2>
          {events && events.length > 0 ? (
            <div className="space-y-2 max-h-48 overflow-y-auto">
              {events.slice(0, 5).map((event: Event) => (
                <div key={event.id} className="text-sm border-b border-slate-700 pb-2">
                  <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${
                    event.severity === 'error' ? 'bg-red-900/50 text-red-300' :
                    event.severity === 'warning' ? 'bg-yellow-900/50 text-yellow-300' :
                    'bg-blue-900/50 text-blue-300'
                  }`}>
                    {event.severity}
                  </span>
                  <span className="text-slate-300 ml-2">{event.message}</span>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-slate-400">No recent events</p>
          )}
        </div>
      </div>
    </div>
  );
};

export default Dashboard;
