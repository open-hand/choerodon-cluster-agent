package polaris

//var responseUrl = "%s://%s/v1/polaris?cluster_id=%s&token=%s"
//
//type ScanRequestInfo struct {
//	RecordId  int    `json:"recordId"`
//	Namespace string `json:"namespace"`
//}
//
//type Result struct {
//	AuditData validator.AuditData    `json:"auditData"`
//	Summary   validator.CountSummary `json:"summary"`
//}
//type ResponseInfo struct {
//	RecordId      int    `json:"recordId"`
//	PolarisResult Result `json:"polarisResult"`
//}
//
//func ScanSystem(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
//	if model.RestrictedModel {
//		return nil, nil
//	}
//	var req ScanRequestInfo
//	err := json.Unmarshal([]byte(cmd.Payload), &req)
//	if err != nil {
//		glog.Info(err.Error())
//		return nil, command.NewResponseError(cmd.Key, model.PolarisRequest, err)
//	}
//	provider, err := kube.CreateResourceProviderFromAPI(opts.KubeClient.GetKubeClient(), "", req.Namespace)
//	if err != nil {
//		glog.Info(err.Error())
//		return nil, command.NewResponseError(cmd.Key, model.PolarisRequest, err)
//	}
//	auditData, err := validator.RunAudit(*opts.PolarisConfig, provider)
//	if err != nil {
//		glog.Info(err.Error())
//		return nil, command.NewResponseError(cmd.Key, model.PolarisRequest, err)
//	}
//	responseInfo := ResponseInfo{RecordId: req.RecordId, PolarisResult: Result{AuditData: auditData, Summary: auditData.GetSummary()}}
//	rawURL := opts.WsClient.URL()
//	schema := ""
//	if rawURL.Scheme == "ws" {
//		schema = "http"
//	} else {
//		schema = "https"
//	}
//	nowURL := fmt.Sprintf(responseUrl, schema, rawURL.Host, model.ClusterId, opts.Token)
//
//	wp := ResponseInfo{
//		RecordId:      responseInfo.RecordId,
//		PolarisResult: responseInfo.PolarisResult,
//	}
//	jsonByte, err := json.Marshal(wp)
//
//	if err != nil {
//		glog.Info("failed to send polaris info.error is %s", err.Error())
//		return nil, command.NewResponseError(cmd.Key, model.PolarisRequest, err)
//	}
//
//	_, err = http.Post(nowURL, "application/json", bytes.NewReader(jsonByte))
//	if err != nil {
//		glog.Info("failed to send polaris info.error is %s", err.Error())
//		return nil, command.NewResponseError(cmd.Key, model.PolarisRequest, err)
//	}
//	return nil, nil
//}
