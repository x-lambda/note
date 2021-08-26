import os
import joblib
import time
import random
import numpy as np
from sklearn import svm
from PIL import Image
import matplotlib.pyplot as plt


"""
svm训练分类模型识别数字
参考: https://github.com/ECNUHP/SVM
"""


def get_file_list(path):
    """获取指定路径下的png文件
    :param path:
    :return:
    """
    return [os.path.join(path, f) for f in os.listdir(path) if f.endswith(".png")]


def get_img_name_str(img_path):
    """获取图片文件的名称
    :param img_path:
    :return:
    """
    return img_path.split(os.path.sep)[-1]


def img2vector(img_file):
    """将 20px * 20px 的图像数据转换成 1*400 的 numpy 向量
    :param img_file:
    :return:
    """
    img = Image.open(img_file).convert('L')
    img_arr = np.array(img, 'i')                       # 20px * 20px 灰度图像
    img_normalization = np.round(img_arr/255)          # 对灰度进行归一化处理
    img_arr2 = np.reshape(img_normalization, (1, -1))  # 1*400 矩阵
    return img_arr2


def read_and_convert(img_file_list):
    """将指定路径下的图片，转成矩阵，且记录对应的数字
    :param img_file_list:
    :return:
    """
    data_label = []                       # 存放类标签
    data_num = len(img_file_list)
    data_mat = np.zeros((data_num, 400))  # data_num * 400 矩阵
    for i in range(data_num):
        img_name_str = img_file_list[i]
        img_name = get_img_name_str(img_name_str)
        class_tag = img_name.split(".")[0].split("_")[0]   # 得到数字
        data_label.append(class_tag)
        data_mat[i, :] = img2vector(img_name_str)
    return data_mat, data_label


def read_all_data(root: str):
    c_name = ['1', '2', '3', '4', '5', '6', '7', '8', '9']
    train_data_path = os.path.join(root, "mnist_image_num/train/")

    # 扫描第一个目录
    print('scaning', train_data_path+"0")
    list_all = get_file_list(train_data_path+"0")
    data_mat, data_label = read_and_convert(list_all)
    for c in c_name:
        print('scaning', train_data_path+c)
        list_all = get_file_list(train_data_path+c)

        data_mat_, data_label_ = read_and_convert(list_all)
        data_mat = np.concatenate((data_mat, data_mat_), axis=0)
        data_label = np.concatenate((data_label, data_label_), axis=0)

    return data_mat, data_label


def train_svm_model(data_mat, data_label, path, decision='ovr'):
    """训练并存储模型
    :param data_mat:
    :param data_label:
    :param path:
    :param decision:
    :return:
    """
    # TODO 调整参数，获取更精确的模型
    clf = svm.SVC(decision_function_shape=decision)

    # TODO 里面算法具体的实现
    rf = clf.fit(data_mat, data_label)

    # 存储模型
    joblib.dump(rf, path)
    return clf


def test_svm_model(root, model_path):
    """测试训练模型
    :param root:
    :param model_path:
    :return:
    """
    test_root = os.path.join(root, "mnist_image_num/test/")
    c_name = ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9']
    all_err_count = 0
    all_score = 0.0
    all_count = 0

    # 加载模型
    model = joblib.load(model_path)
    pre_start = time.clock()
    for c in c_name:
        path = os.path.join(test_root, c)

        img_list = get_file_list(path)
        data_mat, data_label = read_and_convert(img_list)
        print('test data mat shape: {0}, test data label len: {1}'.format(data_mat.shape, len(data_label)))

        start_time = time.clock()
        result = model.predict(data_mat)
        end_time = time.clock()

        print('Recognition ' + c + ' spent {:.4f}s.'.format((end_time-start_time)))
        err_count = len([x for x in result if x != c])
        print('分类错误个数: {}.'.format(err_count))
        all_err_count += err_count
        all_count += len(result)

        score_start = time.clock()
        score = model.score(data_mat, data_label)
        score_end = time.clock()
        print('computing score spent {:.6f}s.'.format(score_end-score_start))

        all_score += score
        print('score: {:.6f}.'.format(score))
        print('err rate is {:.6f}.'.format((1-score)))
        print("*****************************************************************************")

    pre_end = time.clock()
    print('test all class total spent {:.6f}s.'.format(pre_end-pre_start))
    print('all error count is: {}.'.format(all_err_count))
    print('total count is: {}.'.format(all_count))
    avg_accuracy = all_score / 10.0
    print('average accuracy is: {:.6f}.'.format(avg_accuracy))
    print('average error rate is: {:.6f}.'.format(1-avg_accuracy))


def show_result(root, model_path):
    """随机选择图片测试
    然后识别识别出来的数字
    :param root:
    :param model_path:
    :return:
    """
    matrix = []
    c_name = ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9']
    # 打乱顺序
    random.shuffle(c_name)
    # 选择前4个
    show_imgs = dict()
    for c in c_name[:4]:
        test_root = os.path.join(root, "mnist_image_num/test/")
        path = os.path.join(test_root, c)
        files = get_file_list(path)

        # 取前5张图
        # 文件名--->对应的数字分类
        random.shuffle(files)
        row = []
        for f in files[:5]:
            row.append(f)
            show_imgs[f] = c
        matrix.append(row)

    for num in show_imgs.keys():
        print(num, show_imgs.get(num))

    # 随机选择20张图片和识别的结果，如果识别错误，则显示红色
    # plt.show()
    data_mat = np.zeros((20, 400))  # 20 * 400 矩阵
    data_label = []                 # 存放类标签

    i = 0
    for f in show_imgs.keys():
        data_mat[i, :] = img2vector(f)
        data_label.append(show_imgs.get(f))
        i += 1

    model = joblib.load(model_path)
    # 预测结果 result 保存识别的结果，与 data_label 会有出入，之前测试识别率是 97% 左右
    result = model.predict(data_mat)

    fig, axes = plt.subplots(4, 5, figsize=(5, 5), tight_layout=True)
    for row in range(4):
        for col in range(5):
            image_name = matrix[row][col]
            img = plt.imread(image_name)
            axes[row, col].imshow(img)
            color = "green"
            if result[5*row+col] != data_label[5*row+col]:
                color = "red"
            axes[row, col].set_title(result[5*row+col], color=color, size=14)
            axes[row, col].axis('off')
    plt.show()


def main():
    # 训练集路径
    root = "/Users/ilib0x00000000/Desktop/temp/SVM/data"
    start_time = time.time()
    data_mat, data_label = read_all_data(root)

    # 模型保存的路径
    model_path = '/Users/ilib0x00000000/Desktop/temp/ml/svm.model'
    train_svm_model(data_mat, data_label, model_path, decision='ovr')
    end_time = time.time()
    print('Training spent: ', end_time-start_time)


def main1():
    # 测试集路径
    root = "/Users/ilib0x00000000/Desktop/temp/SVM/data"
    
    # 模型保存的路径
    model_path = '/Users/ilib0x00000000/Desktop/temp/ml/svm.model'
    test_svm_model(root, model_path)


def main2():
    # 测试集路径
    root = "/Users/ilib0x00000000/Desktop/temp/SVM/data"
    
    # 模型保存的路径
    model_path = '/Users/ilib0x00000000/Desktop/temp/ml/svm.model'
    show_result(root, model_path)


if __name__ == "__main__":
    main()
    main1()
    main2()


