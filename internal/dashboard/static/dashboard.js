
document.addEventListener('alpine:init', () => {
    Alpine.data('dashboard', () => ({
        tasks: [],
        statistics: {
            all: 0,
            pending: 0,
            running: 0,
            completed: 0,
            failed: 0,
            deleted: 0
        },
        queues: [],
        queueStats: {}, // additional queue statistics
        totalTasks: 0,
        currentPage: 1,
        pageSize: 10,
        showModal: false,
        selectedTaskData: null,
        filters: {
            queue: '',
            status: '',
            fromDate: '',
            toDate: ''
        },
        charts: {
            statusDistribution: null
        },
        // refresh 
        refreshInterval: 30,
        refreshTimer: null,
        nextRefresh: null,
        updateTimer: null,
        // pagination
        pageSize: 10,


        async init() {
            // load configuration from local storage or apply defaults
            this.loadUserPreferences();          

            await this.loadQueues();
            await this.loadData(true);
           
            this.startRefreshTimer();
        },

        get startIndex() {
            return (this.currentPage - 1) * this.pageSize;
        },

        get hasMorePages() {
            return this.startIndex + this.tasks.length < this.totalTasks;
        },

        loadUserPreferences() {
            // load refresh interval from local storage
            const savedInterval = localStorage.getItem('refreshInterval');
            this.refreshInterval = savedInterval ? parseInt(savedInterval, 10) : 30;
            // pagination
            const savedPageSize = localStorage.getItem('pageSize');
            this.pageSize = savedPageSize ? parseInt(savedPageSize, 10) : 10;
            // queue filter
            const savedQueue = localStorage.getItem('queueFilter');
            this.filters.queue = savedQueue || '';
            // status filter
            const savedStatus = localStorage.getItem('statusFilter');
            this.filters.status = savedStatus || '';
            // from date filter
            const savedFromDate = localStorage.getItem('fromDateFilter');
            this.filters.fromDate = savedFromDate || '';
            // to date filter
            const savedToDate = localStorage.getItem('toDateFilter');
            this.filters.toDate = savedToDate || '';

        },

        getStatusClass(status) {
            const classes = {
                pending: 'bg-yellow-100 text-yellow-800',
                running: 'bg-blue-100 text-blue-800',
                completed: 'bg-green-100 text-green-800',
                failed: 'bg-red-100 text-red-800',
                deleted: 'bg-gray-100 text-gray-800'
            };
            return classes[status] || 'bg-gray-100 text-gray-800';
        },

        formatDate(dateString) {
            return new Date(dateString).toLocaleString();
        },

        async loadQueues() {
            try {
                const response = await fetch('/api/v1/queues');
                if (!response.ok) {
                    throw new Error('Failed to load queues');
                }
                
                const queues = await response.json();
                this.queues = queues.map(queue => ({
                    ...queue,
                    // timeout from Go nanoseconds to a human-readable format
                    displayTimeout: this.formatDuration(queue.task_timeout)
                }));

                // if there is a queue selected dont change it, else ensure there is no queue selected
                this.filters.queue = this.filters.queue || '';
                await this.handleFilterChange();
            } catch (error) {
                this.showError('Error loading queues');
                console.error('Error loading queues:', error);
            }
        },

        formatDuration(durationInNanos) {
            const seconds = durationInNanos / 1e9;
            if (seconds >= 3600) {
                return `${Math.floor(seconds / 3600)}h`;
            } else if (seconds >= 60) {
                return `${Math.floor(seconds / 60)}m`;
            }
            return `${seconds}s`;
        },

        async loadData(refresh) {
            await Promise.all([
                this.loadTasks(),
                this.loadStatistics()
            ]);

            if (refresh) {
                this.updateNextRefresh();
            }
        },

        // method to load statistics
        async loadStatistics() {
            try {
                const queryParams = new URLSearchParams({
                    summary: 'true'
                });

                if (this.filters.queue) queryParams.set('queue', this.filters.queue);
                if (this.filters.status) queryParams.set('status', this.filters.status);
                if (this.filters.fromDate) queryParams.set('from', new Date(this.filters.fromDate).getTime() / 1000);
                if (this.filters.toDate) queryParams.set('to', new Date(this.filters.toDate).getTime() / 1000);

                const response = await fetch(`/api/v1/tasks?${queryParams}`);
                if (!response.ok) throw new Error('Failed to load statistics');

                this.statistics = await response.json();
                this.totalTasks = this.statistics.all;

                // update graphs
                this.$nextTick(() => {
                    this.updateCharts();
                });
                
            } catch (error) {
                this.showError('Error loading statistics');
                console.error('Error loading statistics:', error);
            }
        },

        // method to load tasks
        async loadTasks() {
            try {
                const queryParams = new URLSearchParams({
                    offset: this.startIndex.toString(),
                    limit: this.pageSize.toString()
                });

                if (this.filters.queue) queryParams.set('queue', this.filters.queue);
                if (this.filters.status) queryParams.set('status', this.filters.status);
                if (this.filters.fromDate) queryParams.set('from', new Date(this.filters.fromDate).getTime() / 1000);
                if (this.filters.toDate) queryParams.set('to', new Date(this.filters.toDate).getTime() / 1000);

                const response = await fetch(`/api/v1/tasks?${queryParams}`);
                if (!response.ok) throw new Error('Failed to load tasks');

                this.tasks = await response.json();
            } catch (error) {
                this.showError('Error loading tasks');
                console.error('Error loading tasks:', error);
            }
        },

        // handle filter changes and reload data
        async handleFilterChange() {
            this.currentPage = 1; // set current page to 1
            localStorage.setItem('queueFilter', this.filters.queue);
            localStorage.setItem('statusFilter', this.filters.status);
            await this.loadData(true);
        },

        getSuccessRate() {
            const completed = this.statistics.completed || 0;
            const failed = this.statistics.failed || 0;
            const total = completed + failed;
            if (total === 0) return 0;
            return Math.round((completed / total) * 100);
        },

        getCompletionRate() {
            const completed = this.statistics.completed || 0;
            const failed = this.statistics.failed || 0;
            const total = this.statistics.all || 1;
            return Math.round(((completed + failed) / total) * 100);
        },

        updateCharts() {
            if (this.charts.statusDistribution) {
                this.charts.statusDistribution.destroy();
            }

            const ctx = document.getElementById('statusDistribution');
            if (ctx) {
                this.charts.statusDistribution = new Chart(ctx, {
                    type: 'pie',
                    data: {
                        labels: ['Pending', 'Running', 'Completed', 'Failed', 'Deleted'],
                        datasets: [{
                            data: [
                                this.statistics.pending,
                                this.statistics.running,
                                this.statistics.completed,
                                this.statistics.failed,
                                this.statistics.deleted
                            ],
                            backgroundColor: [
                                '#FCD34D', // pending
                                '#60A5FA', // running
                                '#34D399', // completed
                                '#F87171', // failed
                                '#9CA3AF'  // deleted
                            ]
                        }]
                    },
                    options: {
                        responsive: true,
                        animation: {
                            duration: 0
                        },
                        plugins: {
                            legend: {
                                position: 'bottom'
                            },
                            tooltip: {
                                callbacks: {
                                    label: function(context) {
                                        const label = context.label || '';
                                        const value = context.raw || 0;
                                        const total = context.dataset.data.reduce((a, b) => a + b, 0);
                                        const percentage = Math.round((value / total) * 100);
                                        return `${label}: ${value} (${percentage}%)`;
                                    }
                                }
                            }
                        }
                    }
                });
            }
        },

        async deleteTask(task) {
            if (!confirm(`Are you sure you want to delete task '${task.id}'?`)) {
                return;
            }

            try {
                const response = await fetch(`/api/v1/tasks/${task.id}`, {
                    method: 'DELETE'
                });

                if (!response.ok) throw new Error('Failed to delete task');

                this.showSuccess('Task deleted successfully');
                
                // update data
                await this.loadData(false);
            } catch (error) {
                this.showError('Error deleting task');
                console.error('Error deleting task:', error);
            }
        },

        async retryTask(task) {
            try {
                const response = await fetch(`/api/v1/tasks/${task.id}`, {
                    method: 'PUT',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({
                        status: 'pending',
                        data: task.data
                    })
                });

                if (!response.ok) throw new Error('Failed to retry task');

                this.showSuccess('Task queued for retry');
                
                // update data
                await this.loadData(false);
            } catch (error) {
                this.showError('Error retrying task');
                console.error('Error retrying task:', error);
            }
        },

        showTaskData(task) {
            this.selectedTaskData = task.data;
            this.showModal = true;
        },

        async previousPage() {
            if (this.currentPage > 1) {
                this.currentPage--;
                await this.loadTasks();
            }
        },

        async nextPage() {
            if (this.hasMorePages) {
                this.currentPage++;
                await this.loadTasks();
            }
        },

        showSuccess(message) {
            Toastify({
                text: message,
                duration: 3000,
                gravity: "top",
                position: "right",
                style: {
                    background: "linear-gradient(to right, #00b09b, #96c93d)",
                }
            }).showToast();
        },

        showError(message) {
            Toastify({
                text: message,
                duration: 3000,
                gravity: "top",
                position: "right",
                style: {
                    background: "linear-gradient(to right, #ff5f6d, #ffc371)",
                }
            }).showToast();
        },

        get formatNextRefresh() {
            if (!this.nextRefresh) return '';
            const now = new Date();
            const diff = Math.max(0, Math.ceil((this.nextRefresh - now) / 1000));
            return `${diff}s`;
        },
    
        handleRefreshChange() {
            // Store in localStorage
            localStorage.setItem('refreshInterval', this.refreshInterval.toString());
            
            // reset the timer
            this.startRefreshTimer();
            
            // Show success message
            this.showSuccess(
                this.refreshInterval > 0
                    ? `Auto-refresh set to ${this.refreshInterval} seconds`
                    : 'Auto-refresh disabled'
            );
        },
    
        startRefreshTimer() {
            // clean current timers if any
            if (this.refreshTimer) {
                clearInterval(this.refreshTimer);
                this.refreshTimer = null;
            }
            if (this.updateTimer) {
                clearInterval(this.updateTimer);
                this.updateTimer = null;
            }
    
            // if refresh interval is 0, do not start the timer
            if (this.refreshInterval <= 0) {
                this.nextRefresh = null;
                return;
            }
    
            // data refresh timer
            this.refreshTimer = setInterval(async () => {
                await Promise.all([
                    this.loadTasks(),
                    this.loadStatistics()
                ]);
                this.updateNextRefresh();
            }, this.refreshInterval * 1000);
    
            // time left counter
            this.updateNextRefresh();
            this.updateTimer = setInterval(() => {
                if (this.nextRefresh) {
                    this.$refs.nextRefresh.textContent = this.formatNextRefresh;
                }
            }, 1000);
        },
    
        updateNextRefresh() {
            if (this.refreshInterval > 0) {
                this.nextRefresh = new Date(Date.now() + this.refreshInterval * 1000);
            } else {
                this.nextRefresh = null;
            }
        },

        // pagination
        async handlePageSizeChange() {
            // store in localStorage
            localStorage.setItem('pageSize', this.pageSize.toString());
            
            // reset the current page
            this.currentPage = 1;
            
            // reload data
            await this.loadData(false);
            
            // show notification
            this.showSuccess(`Showing ${this.pageSize} results per page`);
        },

        // helper funtion to check if there are more pages
        get hasMorePages() {
            return this.startIndex + Number(this.pageSize) < this.totalTasks;
        },

        // helper function to calculate the start index
        get startIndex() {
            return (this.currentPage - 1) * Number(this.pageSize);
        },

        destroy() {
            if (this.refreshTimer) {
                clearInterval(this.refreshTimer);
            }
            if (this.updateTimer) {
                clearInterval(this.updateTimer);
            }
        }

    }));
    
});